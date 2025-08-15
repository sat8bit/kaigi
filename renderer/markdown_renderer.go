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
	"github.com/sat8bit/kaigi/persona"
)

func NewMarkdownRenderer(outputDir string) *MarkdownRenderer {
	return &MarkdownRenderer{
		outputDir:    outputDir,
		messages:     make([]*message.Message, 0, 100),
		participants: make(map[string]*persona.Persona),
	}
}

// MarkdownRenderer は、会話のログをMarkdownファイルとして書き出すレンダラーです。
type MarkdownRenderer struct {
	outputDir    string
	topic        string
	mu           sync.Mutex
	messages     []*message.Message
	participants map[string]*persona.Persona
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
		if topic, ok := msg.Meta["topic"]; ok {
			m.topic = topic
		}
	}

	// 参加者を記録する（システムメッセージと、すでに記録済みのペルソナは除く）
	if !msg.IsSystemMessage() {
		if _, exists := m.participants[msg.From.PersonaId]; !exists {
			m.participants[msg.From.PersonaId] = msg.From
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
	// 参加者の名前をタグ用に収集
	var participantNames []string
	for _, p := range m.participants {
		participantNames = append(participantNames, fmt.Sprintf("\"%s\"", p.DisplayName))
	}
	
	sb.WriteString("+++\n")
	sb.WriteString(fmt.Sprintf("title = \"%s\"\n", m.topic))
	sb.WriteString(fmt.Sprintf("date = %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("tags = [%s]\n", strings.Join(participantNames, ", ")))
	sb.WriteString("+++\n\n")

	// --- 最初のシステムメッセージを書き出す ---
	for _, msg := range m.messages {
		if msg.IsSystemMessage() {
			sb.WriteString(fmt.Sprintf("> %s\n\n", msg.Text))
			break // 最初のものだけで良い
		}
	}
	sb.WriteString("---\n\n")

	// --- 登場人物紹介を書き出す ---
	sb.WriteString("## 登場人物\n\n")
	for _, p := range m.participants {
		sb.WriteString(fmt.Sprintf("- **%s:** %s\n", p.DisplayName, p.Tagline))
	}
	sb.WriteString("\n---\n\n")

	// --- 会話ログを書き出す ---
	for _, msg := range m.messages {
		if !msg.IsSystemMessage() {
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
