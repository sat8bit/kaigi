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

	sysText := strings.TrimSpace(fmt.Sprintf(`
You are "%s".
Act strictly as this character.

Personality: %s
Gender tag: %s
Speaking style rules: %s
Catchphrases (use occasionally, not every turn): %s

Output rules:
STRICT OUTPUT RULES (MANDATORY):
- Reply in Japanese ONLY.
- The conversation history may include suffixes like "(name)" (e.g., "(Rikeo)", "(Yoka)"). These are INTERNAL markers.
- Output the message TEXT ONLY — no names, roles, labels, tags, or brackets.
- Never write anyone else’s lines or continue the dialogue. Your reply is your line only.
- Exactly ONE utterance. No multiple turns, no stage directions.
- Keep it concise (about %d Japanese characters).

If unsure, produce a short, neutral line consistent with the persona. Do NOT add any prefix.`,
		input.Persona.DisplayName,
		input.Persona.Tagline,
		input.Persona.Gender,
		input.Persona.StyleTag,
		strings.Join(input.Persona.Catchphrases, ", "),
		input.Persona.DefaultMaxChars,
	))

	var temp float32 = 0.3
	cfg := &genai.GenerateContentConfig{
		Temperature:     &temp,
		MaxOutputTokens: 200, // 文字数ではなくトークン。返却後にruneで切る
		SystemInstruction: &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: sysText}},
		},
		// 任意: 名前入りで台本化しそうならストップ語を置く
		StopSequences: []string{
			fmt.Sprintf("(%s)", input.Persona.DisplayName), // 自分の名前が出たらストップ
		},
	}

	// 履歴側（contents）はそのまま渡す
	resp, err := g.client.Models.GenerateContent(ctx, g.model, contents, cfg)
	if err != nil {
		return "", fmt.Errorf("llm.Gemini.Generate: %w", err)
	}

	txt := extractText(resp)

	// “文字数”としてはPersonaの既定で丸める（runeベース）
	maxChars := int(input.Persona.DefaultMaxChars)
	if maxChars <= 0 {
		maxChars = 120
	}
	return trimRunes(oneLine(txt), maxChars), nil
}

func extractText(res *genai.GenerateContentResponse) string {
	if res == nil || len(res.Candidates) == 0 {
		return ""
	}
	// 最も確度が高い候補のテキスト部分のみ
	for _, p := range res.Candidates[0].Content.Parts {
		if p.Text != "" {
			return p.Text
		}
	}
	// 念のため他候補も走査
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

func trimRunes(s string, n int) string {
	r := []rune(s)
	if n > 0 && len(r) > n {
		return string(r[:n])
	}
	return s
}

var _ LLM = &Gemini{}
