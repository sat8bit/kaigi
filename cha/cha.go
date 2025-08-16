package cha

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/llm"
	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
	"github.com/sat8bit/kaigi/topic"
	"github.com/sat8bit/kaigi/turn"
)

func NewCha(
	ctx context.Context,
	chaId string,
	persona *persona.Persona,
	llmInstance llm.LLM,
	bus bus.Bus,
	turnManager turn.Manager,
	turnProvider turn.TurnProvider,
	topics []*topic.Topic,
) *Cha {
	// 初期lastTalkにランダムな遅延を追加して、起動直後の発話タイミングをずらす
	initialLastTalk := time.Now().Add(
		-time.Duration(persona.MinGapSeconds)*time.Second,
	).Add(
		-time.Duration(rand.Intn(5000))*time.Millisecond, // 0〜5秒のランダムな遅延
	)

	return &Cha{
		Context:      ctx,
		ChaId:        chaId,
		Persona:      persona,
		inbox:        make([]*message.Message, 0, 10),
		lastTalk:     initialLastTalk,
		llm:          llmInstance,
		bus:          bus,
		turnManager:  turnManager,
		turnProvider: turnProvider,
		topics:       topics,
	}
}

type Cha struct {
	Context      context.Context
	ChaId        string
	Persona      *persona.Persona
	llm          llm.LLM
	turnManager  turn.Manager
	turnProvider turn.TurnProvider
	bus          bus.Bus
	topics       []*topic.Topic

	mu       sync.Mutex
	inbox    []*message.Message
	lastTalk time.Time
	stopped  bool
}

func (c *Cha) End() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
}

func (c *Cha) Start() {
	messageCh := c.bus.Subscribe()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.Context.Done():
				return

			case in, ok := <-messageCh:
				if !ok {
					return
				}
				c.mu.Lock()
				c.inbox = append(c.inbox, in)
				if len(c.inbox) > 10 {
					c.inbox = c.inbox[1:]
				}
				c.mu.Unlock()

			case <-ticker.C:
				c.tryToTalk()
			}
		}
	}()
}

func (c *Cha) tryToTalk() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	// --- 意思決定フェーズ ---
	c.mu.Lock()
	if time.Since(c.lastTalk).Seconds() < float64(c.Persona.MinGapSeconds) {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	// --- 発話権獲得フェーズ ---
	if err := c.turnManager.Acquire(c.Context); err != nil {
		return
	}
	defer c.turnManager.Release()

	// --- 発話実行フェーズ ---
	c.mu.Lock()
	inboxCopy := make([]*message.Message, len(c.inbox))
	copy(inboxCopy, c.inbox)
	c.mu.Unlock()

	resp, err := c.llm.Generate(c.Context, llm.GenerateInput{
		ChaId:          c.ChaId,
		Persona:        c.Persona,
		RecentMessages: inboxCopy,
		CurrentTurn:    c.turnProvider.GetCurrentTurn(),
		MaxTurns:       c.turnProvider.GetMaxTurns(),
		Topics:         c.topics,
	})

	if err != nil {
		slog.ErrorContext(c.Context, fmt.Sprintf("Cha %s: LLM error: %v", c.ChaId, err))
		// ★ エラー発生時に、エラーメッセージをブロードキャストする
		if berr := c.bus.Broadcast(&message.Message{
			From: c.Persona,
			Text: fmt.Sprintf("LLM error: %v", err),
			At:   time.Now(),
			Kind: message.KindError,
		}); berr != nil {
			slog.ErrorContext(c.Context, fmt.Sprintf("Cha %s: Broadcast error on LLM error: %v", c.ChaId, berr))
		}
		return
	}

	now := time.Now()
	c.mu.Lock()
	c.lastTalk = now
	c.mu.Unlock()

	if err := c.bus.Broadcast(&message.Message{
		From: c.Persona,
		Text: resp,
		At:   now,
	}); err != nil {
		slog.ErrorContext(c.Context, fmt.Sprintf("Cha %s: Broadcast error: %v", c.ChaId, err))
	}
}
