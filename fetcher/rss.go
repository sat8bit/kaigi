package fetcher

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/mmcdole/gofeed"
	"github.com/sat8bit/kaigi/topic"
)

// RSSFetcher は topic.Fetcher インターフェースのRSS実装です。
type RSSFetcher struct {
	url   string
	limit int
}

// NewRSSFetcher は新しい RSSFetcher を生成します。
// limit は取得する記事の上限数を指定します。0以下の場合は無制限。
func NewRSSFetcher(url string, limit int) topic.Fetcher {
	return &RSSFetcher{
		url:   url,
		limit: limit,
	}
}

// Fetch は指定されたURLからRSSフィードを取得し、*topic.Topicのスライスに変換します。
func (f *RSSFetcher) Fetch(ctx context.Context) ([]*topic.Topic, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURLWithContext(f.url, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed from %s: %w", f.url, err)
	}

	// 念のため、公開日で降順にソートして最新のものを取得しやすくする
	sort.Slice(feed.Items, func(i, j int) bool {
		iTime := feed.Items[i].PublishedParsed
		jTime := feed.Items[j].PublishedParsed
		if iTime == nil || jTime == nil {
			return i < j
		}
		return iTime.After(*jTime)
	})

	var topics []*topic.Topic
	for i, item := range feed.Items {
		if f.limit > 0 && i >= f.limit {
			break
		}

		// --- 「門番」ロジック ---
		// 1. HTMLタグを除去
		plainTextSummary := stripHTML(item.Description)
		// 2. 指定文字数で切り捨て
		truncatedSummary := truncateString(plainTextSummary, 200)
		// --- ここまで ---

		topics = append(topics, &topic.Topic{
			Title:     item.Title,
			Summary:   truncatedSummary, // クリーンになった要約をセット
			SourceURL: item.Link,
		})
	}

	return topics, nil
}

// stripHTML は文字列からHTMLタグを削除します。
var htmlRegex = regexp.MustCompile("<[^>]*>")

func stripHTML(s string) string {
	return htmlRegex.ReplaceAllString(s, "")
}

// truncateString は文字列をrune単位で指定された長さに切り詰めます。
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}
