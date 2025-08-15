package supervisor

import (
	"context"
	"sync"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/turn"
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
			if s.turnCount >= s.maxTurns {
				s.cancelFunc() // 上限に達したので停止信号を送る
				s.mu.Unlock()
				return
			}
			s.mu.Unlock()
		}
	}()
}

// GetCurrentTurn は、現在のターン数を返します。
func (s *Supervisor) GetCurrentTurn() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.turnCount
}

// GetMaxTurns は、最大ターン数を返します。
func (s *Supervisor) GetMaxTurns() int {
	// maxTurnsは不変なのでロックは不要
	return s.maxTurns
}

// _ は、*Supervisorがturn.TurnProviderインターフェースを実装していることをコンパイル時に保証します。
var _ turn.TurnProvider = (*Supervisor)(nil)
