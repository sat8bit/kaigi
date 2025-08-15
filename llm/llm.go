package llm

import (
	"context"

	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
	"github.com/sat8bit/kaigi/topic"
)

type LLM interface {
	// Generate generates text based on the provided prompt.
	Generate(ctx context.Context, input GenerateInput) (string, error)
}

type GenerateInput struct {
	ChaId          string
	Persona        *persona.Persona
	RecentMessages []*message.Message // 直近のメッセージ（最大10件）
	CurrentTurn    int                // 現在のターン数
	MaxTurns       int                // 最大ターン数
	Topics         []*topic.Topic     // その会話の「話題」
}
