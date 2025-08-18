package renderer

import (
	"sync"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/persona"
)

// Renderer は、会話のレンダリングを行うコンポーネントが満たすべきインターフェースです。
type Renderer interface {
	// Render は、会話のメインループ中のレンダリング処理を開始します。
	Render(bus bus.Bus, wg *sync.WaitGroup) error

	// Finalize は、すべての会話が終了した後の最終処理を行います。
	// 例えば、ファイルの末尾にフッターを追記するなどの処理を想定しています。
	Finalize(allPersonas []*persona.Persona) error
}
