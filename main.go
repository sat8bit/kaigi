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
	rand.Seed(time.Now().UnixNano())

	// --- フラグ定義 ---
	var (
		maxTurns      = flag.Int("turns", 20, "Maximum number of turns before shutdown")
		personaIDsStr = flag.String("chas", "", "Comma-separated list of persona IDs to participate (e.g., aoi,haru,gou)")
		numChas       = flag.Int("num-chas", 3, "Number of random Chas to participate (used if -chas is not provided)")
		renderersStr  = flag.String("renderers", "console", "Comma-separated list of renderers to use (console, markdown)")
		rssURL        = flag.String("rss-url", "", "URL of the RSS feed to use as a topic")
		rssLimit      = flag.Int("rss-limit", 1, "Maximum number of RSS items to fetch")
		outputDir     = flag.String("output", "./pages/content/posts", "Directory to save markdown files")
		dataDir       = flag.String("data", "./data", "Directory for dynamic data like relationships")
	)
	flag.Parse()

	// --- コンテキストとシグナルハンドリング ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	// --- 環境変数 ---
	projectId := os.Getenv("GOOGLE_CLOUD_PROJECT_ID")
	if projectId == "" {
		log.Fatal("set GOOGLE_CLOUD_PROJECT_ID environment variable")
	}
	location := os.Getenv("LOCATION")
	if location == "" {
		log.Fatal("set LOCATION environment variable")
	}

	// --- 初期化処理 ---
	topics, err := buildTopics(ctx, *rssURL, *rssLimit)
	if err != nil {
		log.Fatalf("failed to build topics: %v", err)
	}

	// --- 主要コンポーネントの初期化 ---
	bus := buspkg.NewMemoryBus()
	turnManager := turn.NewMutexManager()
	var wg sync.WaitGroup

	// 1. レンダラーを構築
	activeRenderers := buildRenderers(*renderersStr, *outputDir, topics)

	// 2. レンダラーを起動
	for _, r := range activeRenderers {
		if err := r.Render(bus, &wg); err != nil {
			log.Fatalf("failed to start renderer: %v", err)
		}
	}

	sup := supervisor.NewSupervisor(*maxTurns, bus, cancel)
	sup.Start()

	// --- Chaの起動 ---
	personaPool, err := persona.NewPool()
	if err != nil {
		log.Fatalf("failed to load persona pool: %v", err)
	}

	personas, err := buildPersonas(personaPool, *personaIDsStr, *numChas)
	if err != nil {
		log.Fatalf("failed to build personas: %v", err)
	}

	relationshipStore := persona.NewRelationshipStore(*dataDir)
	for _, p := range personas {
		if err := relationshipStore.LoadForPersona(p); err != nil {
			slog.Error("failed to load relationship for persona", "personaId", p.PersonaId, "error", err)
		}
	}
	slog.Info("Successfully loaded static personas and dynamic relationships.")

	var chas []*cha.Cha
	var personaNames []string
	for _, p := range personas {
		llmClient := llm.NewGemini(ctx, projectId, location, "gemini-2.5-flash-lite")
		chaInstance := cha.NewCha(ctx, "cha-"+p.PersonaId, p, llmClient, bus, turnManager, sup, topics)
		chas = append(chas, chaInstance)
		personaNames = append(personaNames, p.DisplayName)
		chaInstance.Start()
		slog.Info("Started Cha", "personaId", p.PersonaId, "displayName", p.DisplayName)
	}

	// --- 会話開始 ---
	if err := bus.Broadcast(&message.Message{
		Kind: message.KindSystem,
		Text: fmt.Sprintf("参加者は %s の計 %d 名です。", strings.Join(personaNames, "、"), len(personas)),
	}); err != nil {
		panic(fmt.Errorf("failed to broadcast initial message: %w", err))
	}

	<-ctx.Done()

	// --- 終了処理 ---
	for _, c := range chas {
		c.End()
	}
	bus.Close()
	wg.Wait()

	slog.Info("Finalizing renderers...")
	for _, r := range activeRenderers {
		if err := r.Finalize(personas); err != nil {
			slog.Error("failed to finalize renderer", "error", err)
		}
	}

	slog.Info("Saving all persona relationships...")
	for _, p := range personas {
		if err := relationshipStore.SaveForPersona(p); err != nil {
			slog.Error("failed to save relationship for persona", "personaId", p.PersonaId, "error", err)
		}
	}

	slog.Info("All components shut down gracefully.")
}

func buildTopics(ctx context.Context, rssURL string, rssLimit int) ([]*topic.Topic, error) {
	if rssURL == "" {
		return nil, nil
	}
	slog.Info("Fetching topics from RSS feed...", "url", rssURL)
	topicFetcher := fetcher.NewRSSFetcher(rssURL, rssLimit)
	topics, err := topicFetcher.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch topics: %w", err)
	}
	for _, t := range topics {
		slog.Info("Topic fetched", "title", t.Title)
	}
	return topics, nil
}

func buildPersonas(pool *persona.Pool, personaIDsStr string, numChas int) ([]*persona.Persona, error) {
	if personaIDsStr != "" {
		ids := strings.Split(personaIDsStr, ",")
		var personas []*persona.Persona
		for _, id := range ids {
			cleanID := strings.TrimSpace(id)
			p, err := pool.GetByPersonaId(cleanID)
			if err != nil {
				return nil, fmt.Errorf("failed to find persona with id '%s': %w", cleanID, err)
			}
			personas = append(personas, p)
		}
		return personas, nil
	}

	return pool.GetRandomN(numChas)
}

// ★★★ Renderの呼び出しを削除し、責務を明確化 ★★★
func buildRenderers(renderersStr, outputDir string, topics []*topic.Topic) []renderer.Renderer {
	var activeRenderers []renderer.Renderer

	rendererNames := strings.Split(renderersStr, ",")
	for _, rName := range rendererNames {
		cleanRName := strings.TrimSpace(rName)
		if cleanRName == "" {
			continue
		}

		slog.Info("Building renderer...", "name", cleanRName)
		switch cleanRName {
		case "console":
			activeRenderers = append(activeRenderers, renderer.NewConsoleRenderer())
		case "markdown":
			activeRenderers = append(activeRenderers, renderer.NewMarkdownRenderer(outputDir, topics))
		default:
			slog.Warn("Unknown renderer specified, skipping.", "name", cleanRName)
			continue
		}
	}
	return activeRenderers
}
