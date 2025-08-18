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

const markdownTemplate = `+++
title = {{ .Title }}
date = {{ .Date }}
tags = {{ .Tags }}
+++

{{ .Body }}
`

func NewMarkdownRenderer(outputDir string, topics []*topic.Topic) *MarkdownRenderer {
	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		panic(fmt.Errorf("failed to load JST location: %w", err))
	}
	nowInJST := time.Now().In(jst)
	slug := nowInJST.Format("20060102-150405")
	filePath := filepath.Join(outputDir, slug+".md")

	return &MarkdownRenderer{
		outputDir: outputDir,
		topics:    topics,
		filePath:  filePath,
	}
}

type MarkdownRenderer struct {
	outputDir string
	topics    []*topic.Topic
	filePath  string
}

// ★★★ メソッド名を Finalize に変更 ★★★
func (r *MarkdownRenderer) Finalize(allPersonas []*persona.Persona) error {
	if r.filePath == "" {
		slog.Info("Markdown file path not set, skipping epilogue.")
		return nil
	}

	var epilogue strings.Builder
	epilogue.WriteString("\n---\n\n## 最終的なキャラクターの関係性\n\n")

	personaIdToName := make(map[string]string)
	for _, p := range allPersonas {
		personaIdToName[p.PersonaId] = p.DisplayName
	}

	sort.Slice(allPersonas, func(i, j int) bool {
		return allPersonas[i].DisplayName < allPersonas[j].DisplayName
	})

	for _, p := range allPersonas {
		epilogue.WriteString(fmt.Sprintf("### %s の視点\n", p.DisplayName))

		if len(p.Relationships) == 0 {
			epilogue.WriteString("- (誰とも関係を築かなかった)\n")
		} else {
			targetIds := make([]string, 0, len(p.Relationships))
			for id := range p.Relationships {
				targetIds = append(targetIds, id)
			}
			sort.Slice(targetIds, func(i, j int) bool {
				return personaIdToName[targetIds[i]] < personaIdToName[targetIds[j]]
			})

			for _, targetId := range targetIds {
				rel := p.Relationships[targetId]
				targetName, ok := personaIdToName[targetId]
				if !ok {
					continue
				}
				epilogue.WriteString(fmt.Sprintf("- **%sに対して:** 親密度 `%d` (印象: %s)\n", targetName, rel.Affinity, rel.Impression))
			}
		}
		epilogue.WriteString("\n")
	}

	f, err := os.OpenFile(r.filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("Markdown file does not exist, cannot append epilogue.", "path", r.filePath)
			return nil
		}
		return fmt.Errorf("failed to open markdown file for appending: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(epilogue.String()); err != nil {
		return fmt.Errorf("failed to append epilogue to markdown file: %w", err)
	}

	slog.Info("Epilogue appended to markdown file", "path", r.filePath)
	return nil
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

	title := "Kaigi Log"
	if len(r.topics) > 0 {
		title = r.topics[0].Title
	}

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

	var body strings.Builder

	if systemAnnounce != "" {
		body.WriteString(systemAnnounce)
		body.WriteString("\n---\n\n")
	}

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

	body.WriteString("## 今日の雑談\n\n")
	body.WriteString(conversationLog.String())

	if len(r.topics) > 0 {
		body.WriteString("---\n\n")
		body.WriteString("## 今日の話題\n\n")
		for _, t := range r.topics {
			body.WriteString(fmt.Sprintf("- [%s](%s)\n", t.Title, t.SourceURL))
		}
		body.WriteString("\n")
	}

	var tags []string
	for _, p := range participantsList {
		tags = append(tags, fmt.Sprintf(`"%s"`, p.DisplayName))
	}

	tmpl, err := template.New("markdown").Parse(markdownTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse markdown template: %w", err)
	}

	data := struct {
		Date  string
		Title string
		Tags  string
		Body  string
	}{
		Date:  fmt.Sprintf(`"%s"`, nowInJST.Format("2006-01-02T15:04:05-07:00")),
		Title: fmt.Sprintf(`"%s"`, title),
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

	if err := os.WriteFile(r.filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	slog.Info("Markdown file generated", "path", r.filePath)
	return nil
}
