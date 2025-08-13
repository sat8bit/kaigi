package turn

import (
	"context"
)

// Manager は会話のターンを管理します。
type Manager interface {
	Acquire(ctx context.Context) error
	Release()
}
