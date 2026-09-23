package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	mcpcenterapp "github.com/wanstu/mcp-center"
	"github.com/wanstu/mcp-center/internal/server"
)

func main() {
	defaultDataDir, err := defaultDataDir()
	if err != nil {
		log.Fatal(err)
	}

	listen := flag.String("listen", "127.0.0.1:8610", "HTTP listen address")
	baseURL := flag.String("base-url", "http://127.0.0.1:8610", "External base URL used by OAuth and MCP metadata")
	dataDir := flag.String("data-dir", defaultDataDir, "Persistent data directory")
	flag.Parse()

	assets, err := mcpcenterapp.Assets()
	if err != nil {
		log.Fatalf("load web assets: %v", err)
	}
	app, err := server.New(server.Config{
		DataDir: *dataDir,
		BaseURL: *baseURL,
		Assets:  assets,
	})
	if err != nil {
		log.Fatalf("create MCP Center: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv, listener, err := app.ListenAndServe(ctx, *listen)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	fmt.Printf("MCP Center listening on http://%s\n", listener.Addr())
	fmt.Printf("External base URL: %s\n", app.BaseURL())
	fmt.Printf("Data directory: %s\n", *dataDir)

	<-ctx.Done()
	_ = srv.Close()
}

func defaultDataDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(root, "mcp-center"), nil
}
