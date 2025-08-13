package message

import (
	"time"

	"github.com/sat8bit/kaigi/persona"
)

type Message struct {
	From *persona.Persona
	Text string
	At   time.Time
	Kind Kind // "say", "system", "error", etc.
	Meta map[string]string
}

func (m *Message) IsSystemMessage() bool {
	return m.Kind == KindSystem
}
