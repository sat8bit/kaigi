package cha

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/llm"
	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
	"github.com/sat8bit/kaigi/turn"
)

func NewCha(
	ctx context.Context,
	chaId string,
	persona *persona.Persona,
	llmInstance llm.LLM,
	bus bus.Bus,
	turnManager turn.Manager,
) *Cha {
	return &Cha{
		Context:     ctx,
		ChaId:       chaId,
		Persona:     persona,
		inbox:       make([]*message.Message, 0, 5),
		lastTalk:    time.Now(),
		llm:         llmInstance,
		bus:         bus,
		turnManager: turnManager,
	}
}

type Cha struct {
	Context     context.Context
	ChaId       string
	Persona     *persona.Persona
	llm         llm.LLM
	turnManager turn.Manager
	bus         bus.Bus

	mu       sync.Mutex
	inbox    []*message.Message
	lastTalk time.Time
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
				if len(c.inbox) > 5 {
					c.inbox = c.inbox[1:]
				}
				c.mu.Unlock()

			case <-ticker.C:
				c.tryToTalk()
			}
		}
	}()
}

// tryToTalk は発話条件をチェックし、条件を満たしていれば発話を試みます。
// このメソッドはgoroutineとして安全に呼び出すことができます。
func (c *Cha) tryToTalk() {
	// --- 意思決定フェーズ ---
	c.mu.Lock()
	// 発話するべきか（最低間隔は空いているか）をチェック
	if time.Since(c.lastTalk).Seconds() < float64(c.Persona.MinGapSeconds) {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	// --- 発話権獲得フェーズ ---
	if err := c.turnManager.Acquire(c.Context); err != nil {
		return // コンテキストがキャンセルされたなどで取得に失敗
	}
	defer c.turnManager.Release()

	// --- 発話実行フェーズ ---
	// 発話するので、最終発話時刻を更新

	// LLMに発言を生成させる
	resp, err := c.llm.Generate(c.Context, llm.GenerateInput{
		ChaId:          c.ChaId,
		Persona:        c.Persona,
		RecentMessages: c.inbox,
	})

	if err != nil {
		slog.ErrorContext(c.Context, fmt.Sprintf("Cha %s: LLM error: %v", c.ChaId, err))
		return
	}

	now := time.Now()
	c.lastTalk = now

	// 生成された発言をバスにブロードキャスト
	if err := c.bus.Broadcast(&message.Message{
		From: c.Persona,
		Text: resp,
		At:   now,
		Kind: message.KindSay,
	}); err != nil {
		slog.ErrorContext(c.Context, fmt.Sprintf("Cha %s: Broadcast error: %v", c.ChaId, err))
	}
}
