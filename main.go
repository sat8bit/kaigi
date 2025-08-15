package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	buspkg "github.com/sat8bit/kaigi/bus"
	"github.com/sat8bit/kaigi/cha"
	"github.com/sat8bit/kaigi/fetcher"
	"github.com/sat8bit/kaigi/llm"
	"github.com/sat8bit/kaigi/message"
	"github.com/sat8bit/kaigi/persona"
	"github.com/sat8bit/kaigi/renderer"
	"github.com/sat8bit/kaigi/supervisor"
	"github.com/sat8bit/kaigi/topic"
	"github.com/sat8bit/kaigi/turn"
)

func main() {
	// ★★★ 乱数のシードを初期化 ★★★
	rand.Seed(time.Now().UnixNano())

	// --- コマンドライン引数のパース ---
	var (
		maxTurns  = flag.Int("turns", 20, "Maximum number of turns before shutdown")
		numChas   = flag.Int("chas", 3, "Number of Chas to participate")
		rssURL    = flag.String("rss-url", "", "URL of the RSS feed to use as a topic")
		rssLimit  = flag.Int("rss-limit", 1, "Maximum number of RSS items to fetch")
		outputDir = flag.String("output", "./pages/content/posts", "Directory to save markdown files") // ★ 追加
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

	// --- 話題の取得 ---
	var topics []*topic.Topic
	var err error
	if *rssURL != "" {
		slog.Info("Fetching topics from RSS feed...", "url", *rssURL)
		topicFetcher := fetcher.NewRSSFetcher(*rssURL, *rssLimit)
		topics, err = topicFetcher.Fetch(ctx)
		if err != nil {
			log.Fatalf("failed to fetch topics: %v", err)
		}
		for _, t := range topics {
			slog.Info("Topic fetched", "title", t.Title)
		}
	}

	// --- ペルソナを埋め込みリソースから読み込む ---
	personaPool, err := persona.NewPool()
	if err != nil {
		log.Fatalf("failed to load persona pool: %v", err)
	}

	// --- 主要コンポーネントの初期化 ---
	bus := buspkg.NewMemoryBus()
	turnManager := turn.NewMutexManager()
	var wg sync.WaitGroup

	// --- レンダラーを初期化 ---
	consoleRenderer := renderer.NewConsoleRenderer()
	if err := consoleRenderer.Render(bus, &wg); err != nil {
		log.Fatalf("failed to initialize console renderer: %v", err)
	}

	markdownRenderer := renderer.NewMarkdownRenderer(*outputDir, topics) // ★ 変更
	if err := markdownRenderer.Render(bus, &wg); err != nil {
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
	var chas []*cha.Cha
	for _, p := range personas {
		llmClient := llm.NewGemini(ctx, projectId, location, "gemini-2.5-flash-lite")
		chaInstance := cha.NewCha(
			ctx,
			"cha-"+p.PersonaId,
			p,
			llmClient,
			bus,
			turnManager,
			sup,
			topics,
		)
		chas = append(chas, chaInstance)
		personaNames = append(personaNames, p.DisplayName)
		chaInstance.Start()
		slog.Info("Started Cha", "personaId", p.PersonaId, "displayName", p.DisplayName)
	}

	// 参加者のアナウンス
	if err := bus.Broadcast(&message.Message{
		From: &persona.Persona{},
		Text: fmt.Sprintf("参加者は %s の計 %d 名です。", strings.Join(personaNames, "、"), len(personas)),
		At:   time.Now(),
		Kind: message.KindSystem,
	}); err != nil {
		panic(fmt.Errorf("failed to broadcast initial message: %w", err))
	}

	// ctx.Done() 待ち
	<-ctx.Done()

	// 1. すべてのChaに発話の停止を通知
	for _, c := range chas {
		c.End()
	}

	// 2. メッセージバスを閉じて、新しいメッセージの配信を停止
	bus.Close()

	// 3. すべてのレンダラーが処理を完了するのを待つ
	wg.Wait()

	slog.Info("All components shut down gracefully.")
}
