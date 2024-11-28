# MCP-Go

A Go implementation of the Model Context Protocol (MCP), enabling seamless integration between LLM applications and external data sources and tools.

## About MCP

The Model Context Protocol (MCP) is an open protocol that enables seamless integration between LLM applications and external data sources and tools.
Learn more at [modelcontextprotocol.io](https://modelcontextprotocol.io/) and view the specification at [spec.modelcontextprotocol.io](https://spec.modelcontextprotocol.io/).

## Installation

```bash
go get github.com/mark3labs/mcp-go
```

## Creating a Server
```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// CalculationError represents an error during calculation
type CalculationError struct {
	Message string
}

func (e CalculationError) Error() string {
	return e.Message
}

// Calculator implements basic arithmetic operations
type Calculator struct {
	server *server.DefaultServer
}

// NewCalculator creates a new calculator server
func NewCalculator() *Calculator {
	s := server.NewDefaultServer("calculator", "1.0.0")
	calc := &Calculator{server: s}

	// Register calculator tools
	s.HandleCallTool(calc.handleToolCall)
	s.HandleListTools(calc.handleListTools)

	return calc
}

func (c *Calculator) handleListTools(
	ctx context.Context,
	cursor *string,
) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{
		Tools: []mcp.Tool{
			{
				Name:        "add",
				Description: "Add two numbers",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"a": map[string]interface{}{
							"type":        "number",
							"description": "First number",
						},
						"b": map[string]interface{}{
							"type":        "number",
							"description": "Second number",
						},
					},
				},
			},
			{
				Name:        "subtract",
				Description: "Subtract two numbers",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"a": map[string]interface{}{
							"type":        "number",
							"description": "First number",
						},
						"b": map[string]interface{}{
							"type":        "number",
							"description": "Second number",
						},
					},
				},
			},
			{
				Name:        "multiply",
				Description: "Multiply two numbers",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"a": map[string]interface{}{
							"type":        "number",
							"description": "First number",
						},
						"b": map[string]interface{}{
							"type":        "number",
							"description": "Second number",
						},
					},
				},
			},
			{
				Name:        "divide",
				Description: "Divide two numbers",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"a": map[string]interface{}{
							"type":        "number",
							"description": "First number (dividend)",
						},
						"b": map[string]interface{}{
							"type":        "number",
							"description": "Second number (divisor)",
						},
					},
				},
			},
		},
	}, nil
}

func (c *Calculator) handleToolCall(
	ctx context.Context,
	name string,
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	// Extract arguments
	a, ok := args["a"].(float64)
	if !ok {
		return nil, &CalculationError{Message: "parameter 'a' must be a number"}
	}
	b, ok := args["b"].(float64)
	if !ok {
		return nil, &CalculationError{Message: "parameter 'b' must be a number"}
	}

	var result float64

	switch name {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return nil, &CalculationError{Message: "division by zero"}
		}
		result = a / b
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	// Create response
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%.2f", result),
			},
		},
	}, nil
}

func (c *Calculator) Serve() error {
	return server.ServeStdio(c.server)
}

func main() {
	calc := NewCalculator()

	if err := calc.Serve(); err != nil {
		log.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
```

## Creating a Client
```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	// Create a new client instance
	// Using npx to run the filesystem server with /tmp as the only allowed directory
	c, err := client.NewStdioMCPClient(
		"npx",
		"-y",
		"@modelcontextprotocol/server-filesystem",
		"/tmp",
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize the client
	fmt.Println("Initializing client...")
	initResult, err := c.Initialize(
		ctx,
		mcp.ClientCapabilities{},
		mcp.Implementation{
			Name:    "example-client",
			Version: "1.0.0",
		},
		"1.0",
	)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	fmt.Printf(
		"Initialized with server: %s %s\n\n",
		initResult.ServerInfo.Name,
		initResult.ServerInfo.Version,
	)

	// List Tools
	fmt.Println("Listing available tools...")
	tools, err := c.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}
	for _, tool := range tools.Tools {
		fmt.Printf("- %s: %s\n", tool.Name, tool.Description)
	}
	fmt.Println()

	// List allowed directories
	fmt.Println("Listing allowed directories...")
	result, err := c.CallTool(ctx, "list_allowed_directories", nil)
	if err != nil {
		log.Fatalf("Failed to list allowed directories: %v", err)
	}
	printToolResult(result)
	fmt.Println()

	// List /tmp
	fmt.Println("Listing /tmp directory...")
	result, err = c.CallTool(ctx, "list_directory", map[string]interface{}{
		"path": "/tmp",
	})
	if err != nil {
		log.Fatalf("Failed to list directory: %v", err)
	}
	printToolResult(result)
	fmt.Println()

	// Create mcp directory
	fmt.Println("Creating /tmp/mcp directory...")
	result, err = c.CallTool(ctx, "create_directory", map[string]interface{}{
		"path": "/tmp/mcp",
	})
	if err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}
	printToolResult(result)
	fmt.Println()

	// Create hello.txt
	fmt.Println("Creating /tmp/mcp/hello.txt...")
	result, err = c.CallTool(ctx, "write_file", map[string]interface{}{
		"path":    "/tmp/mcp/hello.txt",
		"content": "Hello World",
	})
	if err != nil {
		log.Fatalf("Failed to create file: %v", err)
	}
	printToolResult(result)
	fmt.Println()

	// Verify file contents
	fmt.Println("Reading /tmp/mcp/hello.txt...")
	result, err = c.CallTool(ctx, "read_file", map[string]interface{}{
		"path": "/tmp/mcp/hello.txt",
	})
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	printToolResult(result)

	// Get file info
	fmt.Println("Getting info for /tmp/mcp/hello.txt...")
	result, err = c.CallTool(ctx, "get_file_info", map[string]interface{}{
		"path": "/tmp/mcp/hello.txt",
	})
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	printToolResult(result)
}

// Helper function to print tool results
func printToolResult(result *mcp.CallToolResult) {
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			fmt.Println(textContent.Text)
		} else {
			jsonBytes, _ := json.MarshalIndent(content, "", "  ")
			fmt.Println(string(jsonBytes))
		}
	}
}
```

## Examples

See the examples /examples directory for implementation examples including:

- Filesystem MCP server integration using stdio transport
- More examples coming soon...

## Contributing

I'm not an expert and this is my first Go library, so contributions are very welcome! Whether it's:

- Improving the code quality
- Adding features
- Fixing bugs
- Writing documentation
- Adding examples

Feel free to open issues and PRs. Let's make this library better together.

## License

MIT License
