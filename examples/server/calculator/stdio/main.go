package main

import (
	"log"
	"os"

	"github.com/mark3labs/mcp-go/examples/server/calculator"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create MCP server with tool capabilities
	mcpServer := server.NewMCPServer(
		"calculator",
		"1.0.0",
	)

	// Add calculator tools
	for _, tool := range calculator.Tools {
		mcpServer.AddTool(tool, calculator.Handlers[tool.Name])
	}

	// Start server
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
