package renderer

import "github.com/sat8bit/kaigi/bus"

type Renderer interface {
	Render(bus bus.Bus) error
}
