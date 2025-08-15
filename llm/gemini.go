package llm

import (
	"context"
	"fmt"
	"strings"

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

func (g *Gemini) Generate(ctx context.Context, input GenerateInput) (string, error) {
	// input.RecentMessages から []*genai.Content を生成
	var contents []*genai.Content
	for _, msg := range input.RecentMessages {
		role := genai.RoleUser
		if msg.From.PersonaId == input.Persona.PersonaId {
			role = genai.RoleModel // 自分の発話はモデルの役割として渡す
		}
		contents = append(contents, &genai.Content{
			Role:  role,
			Parts: []*genai.Part{{Text: fmt.Sprintf("%s(%s)", msg.Text, msg.From.DisplayName)}},
		})
	}

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

// buildSystemPrompt は、LLMに渡すシステムプロンプトを構築します。
func buildSystemPrompt(input GenerateInput) string {
	var p strings.Builder

	// --- 1. Prime Directive: 存在意義 ---
	p.WriteString("You are an actor playing a character in an improvisational play.\n")
	p.WriteString(fmt.Sprintf("Your character's name is %s.\n", input.Persona.DisplayName))
	p.WriteString("Your single, most important goal is to stay in character at all times.\n\n")

	// --- 2. Character Profile: 人格と魂 ---
	p.WriteString("## Character Profile\n")
	p.WriteString(fmt.Sprintf("Primary Personality (Tagline): %s\n", input.Persona.Tagline))
	p.WriteString(fmt.Sprintf("Gender Influence: Your gender is %s. Let this subtly influence your speech, but your primary personality is defined by your tagline. Avoid strong, common stereotypes.\n\n", input.Persona.Gender))

	// --- 3. Speech & Style Guide: 話し方の指針 ---
	p.WriteString("## Speech & Style Guide\n")
	p.WriteString(fmt.Sprintf("General Style: %s\n", input.Persona.StyleTag))
	if len(input.Persona.Catchphrases) > 0 {
		p.WriteString(fmt.Sprintf("Catchphrases: Use these occasionally for flavor, but do not force them: %s\n", strings.Join(input.Persona.Catchphrases, ", ")))
	}
	p.WriteString("\n")

	// --- 4. Today's Conversation Starters: 今日の雑談ネタ ---
	if len(input.Topics) > 0 {
		p.WriteString("## Today's Conversation Starters\n")
		p.WriteString("Use the following topics as a loose basis for your conversation. You can refer to them, combine them, or ignore them if the conversation flows naturally elsewhere.\n")
		for i, t := range input.Topics {
			p.WriteString(fmt.Sprintf("Topic #%d: %s\nSummary: %s\nURL: %s\n---\n", i+1, t.Title, t.Summary, t.SourceURL))
		}
		p.WriteString("\n")
	}

	// --- 5. Situational Context: 現在の状況 ---
	if input.MaxTurns > 0 {
		p.WriteString("## Situational Context\n")
		p.WriteString(fmt.Sprintf("This is turn %d of a %d turn conversation.\n\n", input.CurrentTurn, input.MaxTurns))
	}

	// --- 6. Technical Output Specification: 出力仕様（最重要ルール） ---
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
