package renderer

import (
	"sync"

	"github.com/sat8bit/kaigi/bus"
)

type Renderer interface {
	Render(bus bus.Bus, wg *sync.WaitGroup) error
}
