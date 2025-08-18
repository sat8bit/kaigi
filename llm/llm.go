package llm

import (
	"context"

	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
	"github.com/sat8bit/kaigi/topic"
)

// UpdateRelationshipInput は、関係性更新の際にLLMに渡す入力です。
type UpdateRelationshipInput struct {
	Persona       *persona.Persona
	TargetPersona *persona.Persona
	// 評価対象の会話履歴。この会話（特に末尾の発言）が Persona に与えた影響を評価する。
	RecentMessages      []*message.Message
	CurrentRelationship *persona.Relationship
}

type LLM interface {
	Generate(context.Context, GenerateInput) (string, error)
	// UpdateRelationship は、発言を聞いた後の聞き手の感情変化を評価し、
	// 更新された関係性オブジェクトを返します。
	UpdateRelationship(context.Context, *UpdateRelationshipInput) (*persona.Relationship, error)
}

// GenerateInput は、発話生成の際にLLMに渡す入力です。
type GenerateInput struct {
	ChaId          string
	Persona        *persona.Persona
	RecentMessages []*message.Message
	CurrentTurn    int
	MaxTurns       int
	Topics         []*topic.Topic
	Relationships  map[string]*persona.Relationship // 他の参加者への関係性一覧
}
