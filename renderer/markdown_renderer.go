package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
	"github.com/sat8bit/kaigi/topic"
)

func NewMarkdownRenderer(outputDir string, topics []*topic.Topic) *MarkdownRenderer {
	return &MarkdownRenderer{
		outputDir:    outputDir,
		topics:       topics,
		messages:     make([]*message.Message, 0, 100),
		participants: make(map[string]*persona.Persona),
	}
}

// MarkdownRenderer は、会話のログをMarkdownファイルとして書き出すレンダラーです。
type MarkdownRenderer struct {
	outputDir    string
	topics       []*topic.Topic // ★ 今日の話題を保持
	mu           sync.Mutex
	messages     []*message.Message
	participants map[string]*persona.Persona
}

// Render はバスを購読し、会話のログを収集します。
// バスのチャネルが閉じられると、収集したログをファイルに書き出します。
func (m *MarkdownRenderer) Render(bus bus.Bus, wg *sync.WaitGroup) error {
	ch := bus.Subscribe()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for msg := range ch {
			m.addMessage(msg)
		}

		if err := m.writeToFile(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write markdown log: %v\n", err)
		}
	}()

	return nil
}

func (m *MarkdownRenderer) addMessage(msg *message.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

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
	var participantNames []string
	for _, p := range m.participants {
		participantNames = append(participantNames, fmt.Sprintf("\"%s\"", p.DisplayName))
	}

	pageTitle := "自由な雑談"
	if len(m.topics) > 0 {
		pageTitle = m.topics[0].Title
	}

	sb.WriteString("+++\n")
	sb.WriteString(fmt.Sprintf("title = \"%s\"\n", pageTitle))
	sb.WriteString(fmt.Sprintf("date = %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("tags = [%s]\n", strings.Join(participantNames, ", ")))
	sb.WriteString("+++\n\n")

	// --- 最初のシステムメッセージを書き出す ---
	for _, msg := range m.messages {
		if msg.IsSystemMessage() {
			sb.WriteString(fmt.Sprintf("> %s\n\n", msg.Text))
			break
		}
	}
	sb.WriteString("---\n\n")

	// --- 登場人物紹介を書き出す ---
	sb.WriteString("## 登場人物\n\n")
	for _, p := range m.participants {
		sb.WriteString(fmt.Sprintf("- **%s:** %s\n", p.DisplayName, p.Tagline))
	}
	
	// --- 会話ログを書き出す ---
	sb.WriteString("\n---\n\n")
	sb.WriteString("## 今日の雑談\n\n") // ★ 新しい見出し
	for _, msg := range m.messages {
		if !msg.IsSystemMessage() {
			sb.WriteString(fmt.Sprintf("**%s:** %s\n\n", msg.From.DisplayName, msg.Text))
		}
	}

	// --- 今日の話題を最後に追記 ---
	if len(m.topics) > 0 {
		sb.WriteString("\n---\n\n")
		sb.WriteString("## 今日の話題\n\n")
		for _, t := range m.topics {
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", t.Title, t.SourceURL))
		}
	}

	// --- ファイルに書き出す ---
	if err := os.MkdirAll(m.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fileName := fmt.Sprintf("%s.md", time.Now().Format("20060102-150405"))
	filePath := filepath.Join(m.outputDir, fileName)

	return os.WriteFile(filePath, []byte(sb.String()), 0644)
}
