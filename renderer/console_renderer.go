package renderer

import (
	"fmt"
	"sync"
	"time"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/message"
)

func NewConsoleRenderer() *ConsoleRenderer {
	return &ConsoleRenderer{}
}

type ConsoleRenderer struct {
}

func (c *ConsoleRenderer) Render(bus bus.Bus, wg *sync.WaitGroup) error {
	// コンソールに出力するための Subscriber
	ch := bus.Subscribe()

	wg.Add(1) // ★ WaitGroupにレンダリングゴルーチンを登録
	go func() {
		defer wg.Done() // ★ ゴルーチン終了時に完了を通知
		for o := range ch {
			switch o.Kind {
			case message.KindSystem:
				// システムメッセージはそのまま出力
				fmt.Printf("[System] %s\n", o.Text)
			default:
				fmt.Printf("%s: ", o.From.DisplayName)
				// o.Text を rune で切って表示
				for _, r := range o.Text {
					fmt.Print(string(r))
					time.Sleep(50 * time.Millisecond) // 1文字ずつ表示する効果
				}
				fmt.Println()
			}

		}
	}()

	return nil
}
