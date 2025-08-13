package renderer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/message"
)

func NewMarkdownRenderer(outputDir string) *MarkdownRenderer {
	return &MarkdownRenderer{
		outputDir: outputDir,
		messages:  make([]*message.Message, 0, 100),
	}
}

// MarkdownRenderer は、会話のログをMarkdownファイルとして書き出すレンダラーです。
type MarkdownRenderer struct {
	outputDir string
	topic     string
	mu        sync.Mutex
	messages  []*message.Message
}

// Render はバスを購読し、会話のログを収集します。
// context が Done になると、収集したログをファイルに書き出します。
func (m *MarkdownRenderer) Render(ctx context.Context, bus bus.Bus) error {
	ch := bus.Subscribe()

	go func() {
		for {
			select {
			case <-ctx.Done():
				// プログラム終了時にファイルに書き出す
				if err := m.writeToFile(); err != nil {
					fmt.Fprintf(os.Stderr, "failed to write markdown log: %v\n", err)
				}
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				m.addMessage(msg)
			}
		}
	}()

	return nil
}

func (m *MarkdownRenderer) addMessage(msg *message.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 最初のシステムメッセージからトピックを決定する
	if m.topic == "" && msg.IsSystemMessage() {
		// メタデータからトピックを取得する
		if topic, ok := msg.Meta["topic"]; ok {
			m.topic = topic
		}
	}

	m.messages = append(m.messages, msg)
}

func (m *MarkdownRenderer) writeToFile() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.messages) == 0 {
		return nil
	}

	var sb strings.Builder

	// --- HugoのFront Matterを生成 ---
	sb.WriteString("+++\n")
	sb.WriteString(fmt.Sprintf("title = \"%s\"\n", m.topic))
	sb.WriteString(fmt.Sprintf("date = %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("tags = [\"AI-Kaigi\"]\n")
	sb.WriteString("+++\n\n")

	// --- 会話ログをMarkdown形式で生成 ---
	for _, msg := range m.messages {
		if msg.IsSystemMessage() {
			sb.WriteString(fmt.Sprintf("> %s\n\n", msg.Text))
		} else {
			sb.WriteString(fmt.Sprintf("**%s:** %s\n\n", msg.From.DisplayName, msg.Text))
		}
	}

	// --- ファイルに書き出す ---
	if err := os.MkdirAll(m.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// ファイル名を生成（トピックとタイムスタンプ）
	fileName := fmt.Sprintf("%s-%s.md", m.topic, time.Now().Format("20060102-150405"))
	filePath := filepath.Join(m.outputDir, fileName)

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}
