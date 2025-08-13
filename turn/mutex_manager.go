package turn

import (
	"context"
	"fmt"
)

// MutexManager は turn.Manager の実装です。
// 内部でチャネルを使い、ミューテックスとして機能することで排他制御を実現します。
type MutexManager struct {
	// バッファサイズ1のチャネルをセマフォとして利用します。
	// このチャネルに書き込みができればターンを取得し、
	// このチャネルから読み込みができればターンを解放します。
	turnCh chan struct{}
}

// NewMutexManager は新しい MutexManager を生成します。
func NewMutexManager() Manager {
	m := &MutexManager{
		// バッファサイズ1のチャネルを作成します。
		// これにより、同時に1つのゴルーチンだけがターンを取得できます。
		turnCh: make(chan struct{}, 1),
	}
	return m
}

// Acquire はターンを取得します。
// 既に他の誰かがターンを保持している場合、解放されるまでブロックします。
// context.Context を通じて、待機中にキャンセル操作を受け取ることができます。
func (m *MutexManager) Acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		// コンテキストがキャンセルされた場合（タイムアウトやシャットダウンなど）
		return fmt.Errorf("failed to acquire turn: %w", ctx.Err())
	case m.turnCh <- struct{}{}:
		// チャネルに書き込みが成功した場合、ターン取得成功
		return nil
	}
}

// Release は保持しているターンを解放します。
func (m *MutexManager) Release() {
	select {
	case <-m.turnCh:
		// チャネルから読み込むことで、バッファに空きを作り、
		// 他のゴルーチンがターンを取得できるようにする。
	default:
		// もしチャネルが既に空の場合（例えば、AcquireしていないのにReleaseを呼んだ場合）、
		// 何もせず、パニックも起こさない。
	}
}

// コンパイル時に Manager インターフェースを実装していることを保証します。
var _ Manager = (*MutexManager)(nil)
