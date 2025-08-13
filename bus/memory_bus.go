package bus

import (
	"fmt"
	"sync"

	"github.com/sat8bit/kaigi/message"
)

// MemoryBus は bus.Bus インターフェースのインメモリ実装です。
// 内部で購読者のチャネルリストを保持し、ブロードキャストされたメッセージを
// すべての購読者に配送します。
type MemoryBus struct {
	// 購読しているすべてのチャネルのスライス
	subscribers []chan *message.Message

	// subscribers スライスを保護するための読み書きミューテックス
	mu sync.RWMutex

	// バスが閉じられているかどうかを示すフラグ
	isClosed bool
}

// NewMemoryBus は新しい MemoryBus を生成します。
func NewMemoryBus() Bus {
	return &MemoryBus{
		subscribers: make([]chan *message.Message, 0),
	}
}

// Broadcast はメッセージをすべての購読者にブロードキャストします。
// この操作はノンブロッキングです。もし購読者のチャネルバッファが一杯の場合、
// その購読者へのメッセージはドロップされます。
func (b *MemoryBus) Broadcast(m *message.Message) error {
	// 読み取りロックを使用することで、複数のブロードキャストが並行して実行できます。
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.isClosed {
		return fmt.Errorf("bus is closed")
	}

	// すべての購読者にメッセージを送信
	for _, ch := range b.subscribers {
		select {
		case ch <- m:
			// メッセージを正常に送信
		default:
			// 購読者の受信が追いついていない場合。ここではメッセージをドロップする。
			// 必要であれば、ここでログを出力することもできる。
		}
	}

	return nil
}

// Subscribe は新しい購読者を追加し、メッセージを受信するためのチャネルを返します。
func (b *MemoryBus) Subscribe() <-chan *message.Message {
	// 書き込みロックを使用することで、購読者の追加中に他の操作が実行されるのを防ぎます。
	b.mu.Lock()
	defer b.mu.Unlock()

	// 新しい購読者のためのチャネルを作成（バッファを持たせる）
	newSubscriberCh := make(chan *message.Message, 16)

	if b.isClosed {
		// バスが既に閉じられている場合は、閉じたチャネルを返す
		close(newSubscriberCh)
		return newSubscriberCh
	}

	b.subscribers = append(b.subscribers, newSubscriberCh)

	return newSubscriberCh
}

// Close はバスを閉じ、すべての購読者チャネルをクローズします。
// このメソッドはインターフェースには含まれていませんが、アプリケーションの
// 安全なシャットダウンのために役立ちます。
func (b *MemoryBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isClosed {
		b.isClosed = true
		for _, ch := range b.subscribers {
			close(ch)
		}
		// メモリリークを防ぐためにスライスをクリア
		b.subscribers = nil
	}
}

// コンパイル時に Bus インターフェースを実装していることを保証します。
var _ Bus = (*MemoryBus)(nil)
