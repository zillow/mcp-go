package main

import (
	"log"

	"github.com/mark3labs/mcp-go/examples/server/calculator"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create MCP server
	mcpServer := server.NewMCPServer(
		"calculator",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Add calculator tools
	for _, tool := range calculator.Tools {
		mcpServer.AddTool(tool, calculator.Handlers[tool.Name])
	}

	// Create and start SSE server
	sseServer := server.NewSSEServer(mcpServer, "http://localhost:3001")

	log.Printf("Starting calculator server on :3001")
	if err := sseServer.Start(":3001"); err != nil {
		log.Fatalf("Server error: %v\n", err)
	}
}
