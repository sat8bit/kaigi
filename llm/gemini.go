package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
	"google.golang.org/genai"
)

func NewGemini(ctx context.Context, projectId, location, model string) *Gemini {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectId,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		panic(fmt.Errorf("llm.NewGemini: %w", err))
	}

	return &Gemini{
		client: client,
		model:  model,
	}
}

type Gemini struct {
	client *genai.Client
	model  string
}

func (g *Gemini) messagesToContents(personaId string, messages []*message.Message) []*genai.Content {
	var contents []*genai.Content
	for _, msg := range messages {
		role := genai.RoleUser
		switch msg.Kind {
		case message.KindSystem:
			contents = append(contents, &genai.Content{
				Role:  role,
				Parts: []*genai.Part{{Text: fmt.Sprintf("%s", msg.Text)}},
			})
		case message.KindCha:
			if msg.From.PersonaId == personaId {
				role = genai.RoleModel
			}
			contents = append(contents, &genai.Content{
				Role:  role,
				Parts: []*genai.Part{{Text: fmt.Sprintf("%s(%s)", msg.Text, msg.From.DisplayName)}},
			})
		}
	}

	return contents
}

func (g *Gemini) Generate(ctx context.Context, input GenerateInput) (string, error) {
	contents := g.messagesToContents(input.Persona.PersonaId, input.RecentMessages)

	sysText := buildSystemPrompt(input)

	var temp float32 = 0.3
	cfg := &genai.GenerateContentConfig{
		Temperature:     &temp,
		MaxOutputTokens: 200,
		SystemInstruction: &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: sysText}},
		},
		StopSequences: []string{
			fmt.Sprintf("(%s)", input.Persona.DisplayName),
			"()",
		},
	}

	resp, err := g.client.Models.GenerateContent(ctx, g.model, contents, cfg)
	if err != nil {
		return "", fmt.Errorf("llm.Gemini.Generate: %w", err)
	}

	txt := extractText(resp)

	return oneLine(txt), nil
}

func (g *Gemini) UpdateRelationship(ctx context.Context, input *UpdateRelationshipInput) (*persona.Relationship, error) {
	sysText := buildRelationshipSystemPrompt(input)

	contents := g.messagesToContents(input.Persona.PersonaId, input.RecentMessages)

	var temp float32 = 0.1
	cfg := &genai.GenerateContentConfig{
		Temperature:     &temp,
		MaxOutputTokens: 200,
		SystemInstruction: &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: sysText}},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"affinity":   {Type: genai.TypeInteger},
				"impression": {Type: genai.TypeString},
			},
		},
	}

	resp, err := g.client.Models.GenerateContent(ctx, g.model, contents, cfg)
	if err != nil {
		return nil, fmt.Errorf("llm.Gemini.UpdateRelationship: %w", err)
	}

	rawJson := extractText(resp)
	if rawJson == "" {
		return nil, fmt.Errorf("LLM returned empty response for relationship update")
	}

	var parsedResp struct {
		Affinity   int    `json:"affinity"`
		Impression string `json:"impression"`
	}
	if err := json.Unmarshal([]byte(rawJson), &parsedResp); err != nil {
		return nil, fmt.Errorf("failed to parse guaranteed JSON response: %w. raw response: %s", err, rawJson)
	}

	newRel := &persona.Relationship{
		TargetPersonaId: input.TargetPersona.PersonaId,
		Affinity:        parsedResp.Affinity,
		Impression:      parsedResp.Impression,
	}

	return newRel, nil
}

func buildRelationshipSystemPrompt(input *UpdateRelationshipInput) string {
	var p strings.Builder

	p.WriteString("You are a psychological analyst. Your task is to analyze a conversation from the perspective of one character and determine how their impression of another character has changed.\n\n")
	p.WriteString("## Your Point of View (Persona)\n")
	p.WriteString(fmt.Sprintf("You must adopt the personality of **%s**.\n", input.Persona.DisplayName))
	p.WriteString(fmt.Sprintf("Their core personality is: '%s'.\n\n", input.Persona.Tagline))
	p.WriteString("## Target of Analysis (TargetPersona)\n")
	p.WriteString(fmt.Sprintf("You are analyzing your feelings towards **%s**.\n\n", input.TargetPersona.DisplayName))
	p.WriteString("## Current Relationship\n")
	p.WriteString(fmt.Sprintf("This is your current relationship with %s, *before* the latest message in the conversation.\n", input.TargetPersona.DisplayName))
	p.WriteString(fmt.Sprintf("- Current Affinity Score: %d (from -100 for hate to 100 for love, 0 is neutral)\n", input.CurrentRelationship.Affinity))
	p.WriteString(fmt.Sprintf("- Current Impression Summary: \"%s\"\n\n", input.CurrentRelationship.Impression))
	p.WriteString("## Your Task\n")
	p.WriteString(fmt.Sprintf("Read the provided conversation history. Based on the **last message** from **%s** and the overall context, update your affinity score and impression summary for them.\n\n", input.TargetPersona.DisplayName))
	p.WriteString("## Output Specification\n")
	p.WriteString("Your response must be a valid JSON object conforming to the specified schema.\n")
	p.WriteString("### Key: `affinity`\n")
	p.WriteString("- Type: integer\n")
	p.WriteString("- Description: Your updated affinity score for the speaker (-100 to 100).\n")
	p.WriteString("### Key: `impression`\n")
	p.WriteString("- Type: string\n")
	p.WriteString("- **CRITICAL RULE:** The impression must be an abstract summary of the **speaker's personality, thinking style, or emotional state** revealed in their statement. **DO NOT** mention the specific topic of conversation (e.g., 'washing machines', 'AI'). Focus on *how* they think or feel, not *what* they talked about.\n")
	p.WriteString("- Language: Japanese\n")
	p.WriteString("### Examples\n")
	p.WriteString("**BAD (Too specific):** `\"impression\": \"洗濯機の話に興味を示してくれた。\"`\n")
	p.WriteString("**GOOD (Abstracted):** `\"impression\": \"私の話に真剣に耳を傾け、肯定的に捉えてくれる誠実な人だ。\"`\n")
	p.WriteString("**BAD (Too specific):** `\"impression\": \"AIについての彼の意見はユニークだ。\"`\n")
	p.WriteString("**GOOD (Abstracted):** `\"impression\": \"物事を多角的に捉える、面白い視点を持っているようだ。\"`\n")

	return p.String()
}

func buildSystemPrompt(input GenerateInput) string {
	var p strings.Builder

	p.WriteString("You are an actor playing a character in an improvisational play.\n")
	p.WriteString(fmt.Sprintf("Your character's name is %s.\n", input.Persona.DisplayName))
	p.WriteString("Your single, most important goal is to stay in character at all times.\n\n")
	p.WriteString("## Character Profile\n")
	p.WriteString(fmt.Sprintf("Primary Personality (Tagline): %s\n", input.Persona.Tagline))
	p.WriteString(fmt.Sprintf("Gender Influence: Your gender is %s. Let this subtly influence your speech, but your primary personality is defined by your tagline. Avoid strong, common stereotypes.\n\n", input.Persona.Gender))
	p.WriteString("## Speech & Style Guide\n")
	p.WriteString(fmt.Sprintf("General Style: %s\n", input.Persona.StyleTag))
	if len(input.Persona.Catchphrases) > 0 {
		p.WriteString(fmt.Sprintf("Catchphrases: Use these occasionally for flavor, but do not force them: %s\n", strings.Join(input.Persona.Catchphrases, ", ")))
	}
	p.WriteString("\n")

	if len(input.Topics) > 0 {
		p.WriteString("## Today's Conversation Starters\n")
		p.WriteString("Use the following topics as a loose basis for your conversation. You can refer to them, combine them, or ignore them if the conversation flows naturally elsewhere.\n")
		for i, t := range input.Topics {
			p.WriteString(fmt.Sprintf("Topic #%d: %s\nSummary: %s\nURL: %s\n---\n", i+1, t.Title, t.Summary, t.SourceURL))
		}
		p.WriteString("\n")
	}

	if input.MaxTurns > 0 {
		p.WriteString("## Situational Context\n")
		p.WriteString(fmt.Sprintf("This is turn %d of a %d turn conversation.\n\n", input.CurrentTurn, input.MaxTurns))
	}

	// ★★★ 他者との関係性をプロンプトに追加 ★★★
	if len(input.Relationships) > 0 {
		personaIdToName := make(map[string]string)
		for _, msg := range input.RecentMessages {
			if msg.From != nil && msg.From.PersonaId != "" {
				personaIdToName[msg.From.PersonaId] = msg.From.DisplayName
			}
		}

		p.WriteString("## Your Relationships with Others\n")
		p.WriteString("This is your current emotional state towards the other participants. Use this to subtly influence your tone.\n")
		p.WriteString("A high positive affinity means you are friendly and warm. A negative affinity means you might be cold, sarcastic, or dismissive towards that person.\n\n")

		for targetId, rel := range input.Relationships {
			targetName, ok := personaIdToName[targetId]
			if !ok {
				continue // DisplayNameが不明な相手への言及はスキップ
			}
			p.WriteString(fmt.Sprintf("### Towards %s:\n", targetName))
			p.WriteString(fmt.Sprintf("- Affinity: %d\n", rel.Affinity))
			p.WriteString(fmt.Sprintf("- Your private impression of them: \"%s\"\n\n", rel.Impression))
		}
	}

	p.WriteString("## Technical Output Specification\n")
	p.WriteString("Follow these rules STRICTLY. This is mandatory.\n")
	p.WriteString("1.  **The Golden Rule:** Your reply must be the character's dialogue text ONLY.\n")
	p.WriteString(fmt.Sprintf("2.  **How to Follow Rule #1:** A common mistake is to start your reply with a prefix like `(%s):`. This is forbidden. Your reply MUST begin *directly* with the first word of your dialogue.\n", input.Persona.DisplayName))
	p.WriteString("3.  **Language:** Reply in Japanese ONLY.\n")
	p.WriteString(fmt.Sprintf("4.  **Conciseness:** Keep it concise (around %d Japanese characters).\n", input.Persona.DefaultMaxChars))
	p.WriteString("5.  **Single Utterance:** Provide exactly ONE utterance. Do not write a script with multiple lines or other characters' dialogue.\n")

	return p.String()
}

func extractText(res *genai.GenerateContentResponse) string {
	if res == nil || len(res.Candidates) == 0 {
		return ""
	}
	for _, p := range res.Candidates[0].Content.Parts {
		if p.Text != "" {
			return p.Text
		}
	}
	for _, c := range res.Candidates {
		for _, p := range c.Content.Parts {
			if p.Text != "" {
				return p.Text
			}
		}
	}
	return ""
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

var _ LLM = &Gemini{}
