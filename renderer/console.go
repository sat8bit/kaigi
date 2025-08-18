package renderer

import (
	"fmt"
	"sync"
	"time"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
)

func NewConsoleRenderer() *ConsoleRenderer {
	return &ConsoleRenderer{}
}

type ConsoleRenderer struct{}

func (c *ConsoleRenderer) Render(bus bus.Bus, wg *sync.WaitGroup) error {
	ch := bus.Subscribe()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for o := range ch {
			switch o.Kind {
			case message.KindSystem:
				fmt.Printf("[System] %s\n", o.Text)
			default:
				fmt.Printf("%s: ", o.From.DisplayName)
				for _, r := range o.Text {
					fmt.Print(string(r))
					time.Sleep(50 * time.Millisecond)
				}
				fmt.Println()
			}
		}
	}()

	return nil
}

// Finalize は Renderer インターフェースを実装するためのメソッドです。
// ConsoleRenderer では特に何も行いません。
func (c *ConsoleRenderer) Finalize(allPersonas []*persona.Persona) error {
	return nil
}
