package main

import (
	"log"
	"os"

	example "github.com/mark3labs/mcp-go/examples/server"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create MCP server
	mcpServer := server.NewDefaultServer("calculator", "1.0.0")

	// Register calculator tools
	mcpServer.HandleCallTool(example.HandleToolCall)
	mcpServer.HandleListTools(example.HandleListTools)

	// Start server
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
