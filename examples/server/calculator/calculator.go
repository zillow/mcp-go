package calculator

import (
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
	mcp.NewTool("add",
		mcp.WithDescription("Add two numbers"),
		mcp.WithNumber("a",
			mcp.Required(),
			mcp.Description("First number"),
		),
		mcp.WithNumber("b",
			mcp.Required(),
			mcp.Description("Second number"),
		),
	),
	mcp.NewTool("subtract",
		mcp.WithDescription("Subtract two numbers"),
		mcp.WithNumber("a",
			mcp.Required(),
			mcp.Description("First number"),
		),
		mcp.WithNumber("b",
			mcp.Required(),
			mcp.Description("Second number"),
		),
	),
	mcp.NewTool("multiply",
		mcp.WithDescription("Multiply two numbers"),
		mcp.WithNumber("a",
			mcp.Required(),
			mcp.Description("First number"),
		),
		mcp.WithNumber("b",
			mcp.Required(),
			mcp.Description("Second number"),
		),
	),
	mcp.NewTool("divide",
		mcp.WithDescription("Divide two numbers"),
		mcp.WithNumber("a",
			mcp.Required(),
			mcp.Description("First number (dividend)"),
		),
		mcp.WithNumber("b",
			mcp.Required(),
			mcp.Description("Second number (divisor)"),
		),
	),
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

func HandleAdd(
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	return mcp.FormatNumberResult(a + b), nil
}

func HandleSubtract(
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	return mcp.FormatNumberResult(a - b), nil
}

func HandleMultiply(
	args map[string]interface{},
) (*mcp.CallToolResult, error) {
	a, b, err := extractArgs(args)
	if err != nil {
		return nil, err
	}
	return mcp.FormatNumberResult(a * b), nil
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
	return mcp.FormatNumberResult(a / b), nil
}
