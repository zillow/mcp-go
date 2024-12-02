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

type Tool struct {
	Handler     server.ToolHandlerFunc
	Description string
}

var Tools = map[string]Tool{
	"add": {
		Handler:     HandleAdd,
		Description: "Add two numbers together",
	},
	"subtract": {
		Handler:     HandleSubtract,
		Description: "Subtract the second number from the first",
	},
	"multiply": {
		Handler:     HandleMultiply,
		Description: "Multiply two numbers together",
	},
	"divide": {
		Handler:     HandleDivide,
		Description: "Divide the first number by the second",
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
	name string,
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	return formatResult(a + b), nil
}

func HandleSubtract(
	name string,
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	return formatResult(a - b), nil
}

func HandleMultiply(
	name string,
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	return formatResult(a * b), nil
}

func HandleDivide(
	name string,
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
