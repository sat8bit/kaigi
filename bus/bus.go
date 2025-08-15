package bus

import (
	"github.com/sat8bit/kaigi/message"
)

// Busはメッセージの送受信責務を持つ
type Bus interface {
	Broadcast(m *message.Message) error
	Subscribe() <-chan *message.Message
	Close()
}
