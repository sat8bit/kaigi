package renderer

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
	"github.com/sat8bit/kaigi/topic"
)

// ★ 修正: フロントマターの形式を `+++` に変更
const markdownTemplate = `+++
title = {{ .Title }}
date = {{ .Date }}
tags = {{ .Tags }}
+++

{{ .Body }}
`

func NewMarkdownRenderer(outputDir string, topics []*topic.Topic) *MarkdownRenderer {
	return &MarkdownRenderer{
		outputDir: outputDir,
		topics:    topics,
	}
}

type MarkdownRenderer struct {
	outputDir string
	topics    []*topic.Topic
}

func (r *MarkdownRenderer) Render(bus bus.Bus, wg *sync.WaitGroup) error {
	messageCh := bus.Subscribe()
	var inbox []*message.Message

	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range messageCh {
			inbox = append(inbox, msg)
		}

		for _, msg := range inbox {
			if msg.Kind == message.KindError {
				slog.Info("Error message detected, skipping markdown generation.")
				return
			}
		}

		if len(inbox) < 2 {
			return
		}

		if err := r.render(inbox); err != nil {
			slog.Error("failed to render markdown", "error", err)
		}
	}()

	return nil
}

func (r *MarkdownRenderer) render(inbox []*message.Message) error {
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return fmt.Errorf("failed to load JST location: %w", err)
	}
	nowInJST := time.Now().In(jst)

	slug := nowInJST.Format("20060102-150405")

	title := "Kaigi Log"
	if len(r.topics) > 0 {
		title = r.topics[0].Title
	}

	// --- 1. 必要な情報を収集 ---
	participantsMap := make(map[string]*persona.Persona)
	var conversationLog strings.Builder
	var systemAnnounce string

	for _, msg := range inbox {
		if msg.Kind == message.KindCha {
			if _, ok := participantsMap[msg.From.DisplayName]; !ok {
				participantsMap[msg.From.DisplayName] = msg.From
			}
			conversationLog.WriteString(fmt.Sprintf("**%s**: %s\n\n", msg.From.DisplayName, msg.Text))
		} else if msg.Kind == message.KindSystem {
			systemAnnounce = fmt.Sprintf("> %s\n", msg.Text)
		}
	}

	// --- 2. 本文全体を構築 ---
	var body strings.Builder

	if systemAnnounce != "" {
		body.WriteString(systemAnnounce)
		body.WriteString("\n---\n\n")
	}

	// 登場人物セクション
	participantsList := make([]*persona.Persona, 0, len(participantsMap))
	for _, p := range participantsMap {
		participantsList = append(participantsList, p)
	}
	sort.Slice(participantsList, func(i, j int) bool {
		return participantsList[i].DisplayName < participantsList[j].DisplayName
	})

	body.WriteString("## 登場人物\n\n")
	for _, p := range participantsList {
		body.WriteString(fmt.Sprintf("- **%s:** %s\n", p.DisplayName, p.Tagline))
	}
	body.WriteString("\n---\n\n")

	// 今日の雑談セクション
	body.WriteString("## 今日の雑談\n\n")
	body.WriteString(conversationLog.String())

	// 今日の話題セクション (本文の最後)
	if len(r.topics) > 0 {
		body.WriteString("---\n\n")
		body.WriteString("## 今日の話題\n\n")
		for _, t := range r.topics {
			body.WriteString(fmt.Sprintf("- [%s](%s)\n", t.Title, t.SourceURL))
		}
		body.WriteString("\n")
	}

	// --- 3. フロントマター用のタグを生成 ---
	var tags []string
	for _, p := range participantsList {
		tags = append(tags, fmt.Sprintf(`"%s"`, p.DisplayName)) // ダブルクォートで囲む
	}

	// --- 4. テンプレートに埋め込み ---
	tmpl, err := template.New("markdown").Parse(markdownTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse markdown template: %w", err)
	}

	data := struct {
		Slug  string
		Date  string
		Title string
		Tags  string
		Body  string
	}{
		Slug:  slug,
		Date:  fmt.Sprintf(`"%s"`, nowInJST.Format("2006-01-02T15:04:05-07:00")),
		Title: fmt.Sprintf(`"%s"`, title), // ダブルクォートで囲む
		Tags:  fmt.Sprintf("[%s]", strings.Join(tags, ", ")),
		Body:  body.String(),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	filePath := filepath.Join(r.outputDir, slug+".md")
	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	slog.Info("Markdown file generated", "path", filePath)
	return nil
}
