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
	server server.MCPServer
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
