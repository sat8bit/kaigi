package renderer

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"text/template"
	"time"

	"github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/topic"
)

const markdownTemplate = `---+
slug: "{{ .Slug }}"
date: "{{ .Date }}"
title: "{{ .Title }}"
tags: [{{ .Tags }}]
---

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

		// ★★★ このチェックを追加 ★★★
		// エラーメッセージが含まれている場合は、ファイルを生成しない
		for _, msg := range inbox {
			if msg.Kind == message.KindError {
				slog.Info("Error message detected, skipping markdown generation.")
				return
			}
		}

		if len(inbox) == 0 {
			return
		}

		if err := r.render(inbox); err != nil {
			slog.Error("failed to render markdown", "error", err)
		}
	}()

	return nil
}

func (r *MarkdownRenderer) render(inbox []*message.Message) error {
	slug := time.Now().Format("2006-01-02-150405")
	title := "Kaigi Log"
	if len(r.topics) > 0 {
		title = r.topics[0].Title
	}

	var tags []string
	var body string
	participants := make(map[string]struct{})

	for _, msg := range inbox {
		if msg.Kind == message.KindCha {
			body += fmt.Sprintf("**%s**: %s\n\n", msg.From.DisplayName, msg.Text)
			participants[fmt.Sprintf("'%s'", msg.From.DisplayName)] = struct{}{}
		} else if msg.Kind == message.KindSystem {
			body += fmt.Sprintf("> %s\n\n", msg.Text)
		}
	}

	for p := range participants {
		tags = append(tags, p)
	}
	if len(r.topics) > 0 {
		for _, t := range r.topics {
			tags = append(tags, fmt.Sprintf("'%s'", t.Title))
		}
	}

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
		Date:  time.Now().Format("2006-01-02T15:04:05-07:00"),
		Title: title,
		Tags:  fmt.Sprintf("[%s]", string(bytes.Join([][]byte{[]byte(fmt.Sprintf("'%s'", title))}, []byte(",")))),
		Body:  body,
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
