package topic

import "context"

// Fetcher は、外部のデータソースから、[]*Topic を取得するためのインターフェースです。
type Fetcher interface {
	Fetch(ctx context.Context) ([]*Topic, error)
}
