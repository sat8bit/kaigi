package supervisor

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/sat8bit/kaigi/bus"
)

// Supervisor は、会話のターン数を監視し、上限に達したら停止信号を送ります。
type Supervisor struct {
	maxTurns   int
	turnCount  int
	bus        bus.Bus
	cancelFunc context.CancelFunc
	mu         sync.Mutex
}

// NewSupervisor は、新しい Supervisor を生成します。
func NewSupervisor(maxTurns int, bus bus.Bus, cancelFunc context.CancelFunc) *Supervisor {
	return &Supervisor{
		maxTurns:   maxTurns,
		bus:        bus,
		cancelFunc: cancelFunc,
	}
}

// Start は、会話の監視を開始します。
func (s *Supervisor) Start() {
	ch := s.bus.Subscribe()

	go func() {
		for msg := range ch {
			// システムメッセージはカウントしない
			if msg.IsSystemMessage() {
				continue
			}

			s.mu.Lock()
			s.turnCount++
			fmt.Fprintf(os.Stderr, "[Supervisor] Turn %d/%d\n", s.turnCount, s.maxTurns)
			if s.turnCount >= s.maxTurns {
				fmt.Fprintf(os.Stderr, "[Supervisor] Reached max turns. Shutting down...\n")
				s.cancelFunc() // 上限に達したので停止信号を送る
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
		}
	}()
}
