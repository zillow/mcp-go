package main

import (
	"log"

	example "github.com/mark3labs/mcp-go/examples/server"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create MCP server
	mcpServer := server.NewDefaultServer("calculator", "1.0.0")

	// Register handlers
	mcpServer.HandleCallTool(example.HandleToolCall)
	mcpServer.HandleListTools(example.HandleListTools)

	// Create and start SSE server
	sseServer := server.NewSSEServer(mcpServer, "http://localhost:3001")

	log.Printf("Starting calculator server on :3001")
	if err := sseServer.Start(":3001"); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}
