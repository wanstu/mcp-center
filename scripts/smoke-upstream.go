package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoArgs struct {
	Message string `json:"message" jsonschema:"message to echo"`
}

func main() {
	transport := flag.String("transport", "stdio", "stdio or http")
	listen := flag.String("listen", "127.0.0.1:18611", "HTTP listen address")
	flag.Parse()

	server := mcp.NewServer(&mcp.Implementation{Name: "mcp-center-smoke", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echo a message"}, func(ctx context.Context, req *mcp.CallToolRequest, args echoArgs) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "echo:" + args.Message}}}, nil, nil
	})

	switch *transport {
	case "stdio":
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
	case "http":
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
			Stateless: true, JSONResponse: true,
		})
		log.Printf("smoke HTTP MCP listening on %s", *listen)
		if err := http.ListenAndServe(*listen, handler); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal(fmt.Errorf("unsupported transport %q", *transport))
	}
}
