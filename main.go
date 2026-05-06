package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	up "github.com/jmpa-io/up-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx := context.Background()

	// read token from environment.
	token := os.Getenv("UP_TOKEN")
	if token == "" {
		log.Fatal("UP_TOKEN environment variable is not set")
	}

	// create Up client — validates the token immediately via Ping.
	client, err := up.New(ctx, token, up.WithLogLevel(slog.LevelError))
	if err != nil {
		log.Fatalf("failed to create Up client: %v", err)
	}

	// create MCP server.
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "up",
		Version: "v0.0.1",
	}, nil)

	// register tools.
	registerTools(server, client)

	// run over stdio.
	fmt.Fprintln(os.Stderr, "up-mcp server started")
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server exited with error: %v", err)
	}
}
