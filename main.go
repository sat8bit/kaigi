package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	buspkg "github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/cha"
	"github.com/sat8bit/kaigi/llm"
	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
	"github.com/sat8bit/kaigi/renderer"
	"github.com/sat8bit/kaigi/supervisor"
	"github.com/sat8bit/kaigi/turn"
)

func main() {
	// --- コマンドライン引数のパース ---
	var (
		topic    = flag.String("topic", "今日の天気について", "Initial topic for the conversation")
		maxTurns = flag.Int("turns", 20, "Maximum number of turns before shutdown")
		numChas  = flag.Int("chas", 3, "Number of Chas to participate")
	)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ctrl+C シグナルで cancel()
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	projectId := os.Getenv("PROJECT_ID")
	if projectId == "" {
		log.Fatal("set PROJECT_ID environment variable")
	}
	location := os.Getenv("LOCATION")
	if location == "" {
		log.Fatal("set LOCATION environment variable")
	}

	// --- ペルソナを埋め込みリソースから読み込む ---
	personaPool, err := persona.NewPool()
	if err != nil {
		log.Fatalf("failed to load persona pool: %v", err)
	}

	bus := buspkg.NewMemoryBus()
	turnManager := turn.NewMutexManager()

	// --- レンダラーを初期化 ---
	consoleRenderer := renderer.NewConsoleRenderer()
	if err := consoleRenderer.Render(bus); err != nil {
		log.Fatalf("failed to initialize console renderer: %v", err)
	}

	markdownRenderer := renderer.NewMarkdownRenderer("./pages/content/posts")
	if err := markdownRenderer.Render(ctx, bus); err != nil {
		log.Fatalf("failed to initialize markdown renderer: %v", err)
	}

	// --- Supervisorを初期化して起動 ---
	sup := supervisor.NewSupervisor(*maxTurns, bus, cancel)
	sup.Start()

	// --- 読み込んだペルソナから Cha を生成して起動 ---
	personas, err := personaPool.GetRandomN(*numChas)
	if err != nil {
		log.Fatalf("failed to get random personas: %v", err)
	}
	var personaNames []string
	for _, p := range personas {
		llmClient := llm.NewGemini(ctx, projectId, location, "gemini-2.5-flash-lite")
		chaInstance := cha.NewCha(
			ctx,
			"cha-"+p.PersonaId,
			p,
			llmClient,
			bus,
			turnManager,
		)
		personaNames = append(personaNames, p.DisplayName)
		chaInstance.Start()
		slog.Info("Started Cha", "personaId", p.PersonaId, "displayName", p.DisplayName)
	}

	// 最初の話題を送信
	if err := bus.Broadcast(&message.Message{
		From: &persona.Persona{},
		Text: fmt.Sprintf("話題は「%s」。 参加者は %s の計 %d 名です。", *topic, strings.Join(personaNames, "、"), len(personas)),
		At:   time.Now(),
		Kind: message.KindSystem,
		Meta: map[string]string{
			"topic": *topic,
		},
	}); err != nil {
		panic(fmt.Errorf("failed to broadcast initial message: %w", err))
	}

	// ctx.Done() 待ち
	<-ctx.Done()
	time.Sleep(500 * time.Millisecond) // 残りの出力を拾う余裕
	fmt.Println("")
	fmt.Println("Shutting down...")
}
