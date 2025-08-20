package message

import (
	"time"

	"github.com/sat8bit/kaigi/persona"
)

type Kind string

const (
	KindSystem      Kind = "system"
	KindCha         Kind = "cha"
	KindError       Kind = "error"
	KindEnd         Kind = "end"
	KindTurnChanged Kind = "turn_changed"
	KindLog         Kind = "log" // ★★★ ログメッセージ用のKindを追加 ★★★
)

type Message struct {
	From *persona.Persona
	Text string
	At   time.Time
	Kind Kind
}
