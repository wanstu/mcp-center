package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	mcpcenterapp "github.com/wanstu/mcp-center"
	"github.com/wanstu/mcp-center/internal/buildinfo"
	"github.com/wanstu/mcp-center/internal/server"
)

func main() {
	foregroundArgs, handled, err := dispatchCLI(os.Args[1:], os.Stdout)
	if handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "mcp-center:", err)
			os.Exit(2)
		}
		return
	}
	defaultDataDir, err := defaultDataDir()
	if err != nil {
		log.Fatal(err)
	}

	listen := flag.String("listen", "0.0.0.0:8610", "HTTP listen address (all network interfaces by default)")
	baseURL := flag.String("base-url", "", "Optional canonical public base URL for OAuth metadata (default: localhost; MCP links use the current request address)")
	dataDir := flag.String("data-dir", defaultDataDir, "Persistent data directory")
	showVersion := flag.Bool("version", false, "Print version and exit")
	// Go's built-in flag help must present the same command overview even
	// when -h appears after another foreground option.
	flag.Usage = func() { fmt.Fprint(os.Stdout, commandHelp) }
	if err := flag.CommandLine.Parse(foregroundArgs); err != nil {
		log.Fatalf("invalid flags: %v", err)
	}
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "mcp-center: unexpected arguments: %v (run mcp-center --help)\n", flag.Args())
		os.Exit(2)
	}

	if *showVersion {
		info := buildinfo.Current()
		fmt.Printf("MCP Center %s", info.Version)
		if info.Commit != "" {
			fmt.Printf(" (%s", info.Commit)
			if info.Modified {
				fmt.Print(", modified")
			}
			fmt.Print(")")
		}
		fmt.Println()
		return
	}

	autoBaseURL := *baseURL == ""
	if autoBaseURL {
		*baseURL, err = localBaseURLForListen(*listen)
		if err != nil {
			log.Fatalf("invalid --listen: %v", err)
		}
	}

	assets, err := mcpcenterapp.Assets()
	if err != nil {
		log.Fatalf("load web assets: %v", err)
	}
	app, err := server.New(server.Config{
		DataDir:     *dataDir,
		BaseURL:     *baseURL,
		AutoBaseURL: autoBaseURL,
		Assets:      assets,
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
	info := buildinfo.Current()
	fmt.Printf("MCP Center %s", info.Version)
	if info.Commit != "" {
		fmt.Printf(" (%s)", info.Commit)
	}
	fmt.Println()
	fmt.Printf("MCP Center listening on http://%s\n", listener.Addr())
	if !autoBaseURL {
		fmt.Printf("Canonical OAuth Base URL: %s\n", app.BaseURL())
	} else {
		fmt.Println("Access locally or with any server IP/domain at the listening port.")
	}
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

func localBaseURLForListen(listen string) (string, error) {
	_, port, err := net.SplitHostPort(listen)
	if err != nil || port == "" {
		return "", fmt.Errorf("listen address must include a port: %q", listen)
	}
	return "http://127.0.0.1:" + port, nil
}
