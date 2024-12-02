package calculator

import (
	"fmt"

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

var Handlers = map[string]server.ToolHandlerFunc{
	"add":      HandleAdd,
	"subtract": HandleSubtract,
	"multiply": HandleMultiply,
	"divide":   HandleDivide,
}

var Tools = []mcp.Tool{
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
}

// Helper function to extract and validate number arguments
func extractArgs(args map[string]interface{}) (float64, float64, error) {
	a, ok := args["a"].(float64)
	if !ok {
		return 0, 0, &CalculationError{
			Message: "parameter 'a' must be a number",
		}
	}
	b, ok := args["b"].(float64)
	if !ok {
		return 0, 0, &CalculationError{
			Message: "parameter 'b' must be a number",
		}
	}
	return a, b, nil
}

// Helper function to format result
func formatResult(result float64) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []interface{}{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%.2f", result),
			},
		},
	}
}

func HandleAdd(
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	return formatResult(a + b), nil
}

func HandleSubtract(
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	return formatResult(a - b), nil
}

func HandleMultiply(
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	return formatResult(a * b), nil
}

func HandleDivide(
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	if b == 0 {
		return nil, &CalculationError{Message: "division by zero"}
	}
	return formatResult(a / b), nil
}
