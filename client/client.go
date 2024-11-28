package client

import "context"

// Client represents an MCP client interface
type MCPClient interface {
	// Initialize sends the initial connection request to the server
	Initialize(
		ctx context.Context,
		capabilities ClientCapabilities,
		clientInfo Implementation,
		protocolVersion string,
	) (*InitializeResult, error)

	// Ping checks if the server is alive
	Ping(ctx context.Context) error

	// ListResources requests a list of available resources from the server
	ListResources(
		ctx context.Context,
		cursor *string,
	) (*ListResourcesResult, error)

	// ReadResource reads a specific resource from the server
	ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error)

	// Subscribe requests notifications for changes to a specific resource
	Subscribe(ctx context.Context, uri string) error

	// Unsubscribe cancels notifications for a specific resource
	Unsubscribe(ctx context.Context, uri string) error

	// ListPrompts requests a list of available prompts from the server
	ListPrompts(ctx context.Context, cursor *string) (*ListPromptsResult, error)

	// GetPrompt retrieves a specific prompt from the server
	GetPrompt(
		ctx context.Context,
		name string,
		arguments map[string]string,
	) (*GetPromptResult, error)

	// ListTools requests a list of available tools from the server
	ListTools(ctx context.Context, cursor *string) (*ListToolsResult, error)

	// CallTool invokes a specific tool on the server
	CallTool(
		ctx context.Context,
		name string,
		arguments map[string]interface{},
	) (*CallToolResult, error)

	// SetLevel sets the logging level for the server
	SetLevel(ctx context.Context, level LoggingLevel) error

	// Complete requests completion options for a given argument
	Complete(
		ctx context.Context,
		ref interface{},
		argument CompleteArgument,
	) (*CompleteResult, error)
}
