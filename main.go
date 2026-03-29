package main

import (
	"log"

	"github.com/mark3labs/mcp-go/server"

	"video-pipeline-mcp/config"
	"video-pipeline-mcp/jobs"
	"video-pipeline-mcp/tools"
)

func main() {
	cfg := config.Load()
	queue := jobs.NewQueue(cfg)

	s := server.NewMCPServer(
		"video-pipeline",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	tools.Register(s, cfg, queue)

	log.Println("Video Pipeline MCP server starting (stdio)...")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
