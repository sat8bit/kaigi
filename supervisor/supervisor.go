package supervisor

import (
	"context"
	"log/slog"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/message"
)

func NewSupervisor(maxTurns int, bus bus.Bus, cancel context.CancelFunc) *Supervisor {
	return &Supervisor{
		maxTurns: maxTurns,
		bus:      bus,      // ★ 追加
		cancel:   cancel,
	}
}

type Supervisor struct {
	maxTurns    int
	currentTurn int
	bus         bus.Bus // ★ 追加
	cancel      context.CancelFunc
}

func (s *Supervisor) Start() {
	messageCh := s.bus.Subscribe()

	go func() {
		for msg := range messageCh {
			switch msg.Kind {
			case message.KindError: // ★ 追加
				slog.Error("Error message received, shutting down.", "from", msg.From.DisplayName, "error", msg.Text)
				s.cancel()
				return
			default:
				s.currentTurn++
				if s.currentTurn >= s.maxTurns {
					slog.Info("Max turns reached, shutting down.")
					s.cancel()
					return
				}
			}
		}
	}()
}

func (s *Supervisor) GetCurrentTurn() int {
	return s.currentTurn
}

func (s *Supervisor) GetMaxTurns() int {
	return s.maxTurns
}
