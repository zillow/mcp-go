// Package server provides MCP (Model Control Protocol) server implementations.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/mark3labs/mcp-go/mcp"
)

// resourceEntry holds both a resource and its handler
type resourceEntry struct {
	resource mcp.Resource
	handler  ResourceHandlerFunc
}

// resourceTemplateEntry holds both a template and its handler
type resourceTemplateEntry struct {
	template mcp.ResourceTemplate
	handler  ResourceTemplateHandlerFunc
}

// ServerOption is a function that configures an MCPServer.
type ServerOption func(*MCPServer)

// ResourceHandlerFunc is a function that returns resource contents.
type ResourceHandlerFunc func(request mcp.ReadResourceRequest) ([]interface{}, error)

// ResourceTemplateHandlerFunc is a function that returns a resource template.
type ResourceTemplateHandlerFunc func(request mcp.ReadResourceRequest) ([]interface{}, error)

// PromptHandlerFunc handles prompt requests with given arguments.
type PromptHandlerFunc func(arguments map[string]string) (*mcp.GetPromptResult, error)

// ToolHandlerFunc handles tool calls with given arguments.
type ToolHandlerFunc func(arguments map[string]interface{}) (*mcp.CallToolResult, error)

// NotificationHandlerFunc handles incoming notifications.
type NotificationHandlerFunc func(notification mcp.JSONRPCNotification)

// MCPServer implements a Model Control Protocol server that can handle various types of requests
// including resources, prompts, and tools.
type MCPServer struct {
	name              string
	version           string
	resources         map[string]resourceEntry
	resourceTemplates map[string]resourceTemplateEntry
	prompts           map[string]mcp.Prompt
	promptHandlers    map[string]PromptHandlerFunc
	tools             map[string]mcp.Tool
	toolHandlers      map[string]ToolHandlerFunc
	notifications     []NotificationHandlerFunc
	capabilities      serverCapabilities
}

// serverCapabilities defines the supported features of the MCP server
type serverCapabilities struct {
	resources *resourceCapabilities
	prompts   *promptCapabilities
	logging   bool
}

// resourceCapabilities defines the supported resource-related features
type resourceCapabilities struct {
	subscribe   bool
	listChanged bool
}

// promptCapabilities defines the supported prompt-related features
type promptCapabilities struct {
	listChanged bool
}

// WithResourceCapabilities configures resource-related server capabilities
func WithResourceCapabilities(subscribe, listChanged bool) ServerOption {
	return func(s *MCPServer) {
		s.capabilities.resources = &resourceCapabilities{
			subscribe:   subscribe,
			listChanged: listChanged,
		}
	}
}

// WithPromptCapabilities configures prompt-related server capabilities
func WithPromptCapabilities(listChanged bool) ServerOption {
	return func(s *MCPServer) {
		s.capabilities.prompts = &promptCapabilities{
			listChanged: listChanged,
		}
	}
}

// WithLogging enables logging capabilities for the server
func WithLogging() ServerOption {
	return func(s *MCPServer) {
		s.capabilities.logging = true
	}
}

// NewMCPServer creates a new MCP server instance with the given name, version and options
func NewMCPServer(
	name, version string,
	opts ...ServerOption,
) *MCPServer {
	s := &MCPServer{
		resources:         make(map[string]resourceEntry),
		resourceTemplates: make(map[string]resourceTemplateEntry),
		prompts:           make(map[string]mcp.Prompt),
		promptHandlers:    make(map[string]PromptHandlerFunc),
		tools:             make(map[string]mcp.Tool),
		toolHandlers:      make(map[string]ToolHandlerFunc),
		name:              name,
		version:           version,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// HandleMessage processes an incoming JSON-RPC message and returns an appropriate response
func (s *MCPServer) HandleMessage(
	ctx context.Context,
	message json.RawMessage,
) mcp.JSONRPCMessage {
	var baseMessage struct {
		JSONRPC string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		ID      interface{} `json:"id,omitempty"`
	}

	if err := json.Unmarshal(message, &baseMessage); err != nil {
		return createErrorResponse(
			nil,
			mcp.PARSE_ERROR,
			"Failed to parse message",
		)
	}

	// Check for valid JSONRPC version
	if baseMessage.JSONRPC != mcp.JSONRPC_VERSION {
		return createErrorResponse(
			baseMessage.ID,
			mcp.INVALID_REQUEST,
			"Invalid JSON-RPC version",
		)
	}

	if baseMessage.ID == nil {
		var notification mcp.JSONRPCNotification
		if err := json.Unmarshal(message, &notification); err != nil {
			return createErrorResponse(
				nil,
				mcp.PARSE_ERROR,
				"Failed to parse notification",
			)
		}
		return s.handleNotification(notification)
	}

	switch baseMessage.Method {
	case "initialize":
		var request mcp.InitializeRequest
		if err := json.Unmarshal(message, &request); err != nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.INVALID_REQUEST,
				"Invalid initialize request",
			)
		}
		return s.handleInitialize(baseMessage.ID, request)
	case "ping":
		var request mcp.PingRequest
		if err := json.Unmarshal(message, &request); err != nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.INVALID_REQUEST,
				"Invalid ping request",
			)
		}
		return s.handlePing(baseMessage.ID, request)
	case "resources/list":
		if s.capabilities.resources == nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.METHOD_NOT_FOUND,
				"Resources not supported",
			)
		}
		var request mcp.ListResourcesRequest
		if err := json.Unmarshal(message, &request); err != nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.INVALID_REQUEST,
				"Invalid list resources request",
			)
		}
		return s.handleListResources(baseMessage.ID, request)
	case "resources/templates/list":
		if s.capabilities.resources == nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.METHOD_NOT_FOUND,
				"Resources not supported",
			)
		}
		var request mcp.ListResourceTemplatesRequest
		if err := json.Unmarshal(message, &request); err != nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.INVALID_REQUEST,
				"Invalid list resource templates request",
			)
		}
		return s.handleListResourceTemplates(baseMessage.ID, request)
	case "resources/read":
		if s.capabilities.resources == nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.METHOD_NOT_FOUND,
				"Resources not supported",
			)
		}
		var request mcp.ReadResourceRequest
		if err := json.Unmarshal(message, &request); err != nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.INVALID_REQUEST,
				"Invalid read resource request",
			)
		}
		return s.handleReadResource(baseMessage.ID, request)
	case "prompts/list":
		if s.capabilities.prompts == nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.METHOD_NOT_FOUND,
				"Prompts not supported",
			)
		}
		var request mcp.ListPromptsRequest
		if err := json.Unmarshal(message, &request); err != nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.INVALID_REQUEST,
				"Invalid list prompts request",
			)
		}
		return s.handleListPrompts(baseMessage.ID, request)
	case "prompts/get":
		if s.capabilities.prompts == nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.METHOD_NOT_FOUND,
				"Prompts not supported",
			)
		}
		var request mcp.GetPromptRequest
		if err := json.Unmarshal(message, &request); err != nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.INVALID_REQUEST,
				"Invalid get prompt request",
			)
		}
		return s.handleGetPrompt(baseMessage.ID, request)
	case "tools/list":
		if len(s.tools) == 0 {
			return createErrorResponse(
				baseMessage.ID,
				mcp.METHOD_NOT_FOUND,
				"Tools not supported",
			)
		}
		var request mcp.ListToolsRequest
		if err := json.Unmarshal(message, &request); err != nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.INVALID_REQUEST,
				"Invalid list tools request",
			)
		}
		return s.handleListTools(baseMessage.ID, request)
	case "tools/call":
		if len(s.tools) == 0 {
			return createErrorResponse(
				baseMessage.ID,
				mcp.METHOD_NOT_FOUND,
				"Tools not supported",
			)
		}
		var request mcp.CallToolRequest
		if err := json.Unmarshal(message, &request); err != nil {
			return createErrorResponse(
				baseMessage.ID,
				mcp.INVALID_REQUEST,
				"Invalid call tool request",
			)
		}
		return s.handleToolCall(baseMessage.ID, request)
	default:
		return createErrorResponse(
			baseMessage.ID,
			mcp.METHOD_NOT_FOUND,
			fmt.Sprintf("Method %s not found", baseMessage.Method),
		)
	}
}

// AddResource registers a new resource and its handler
func (s *MCPServer) AddResource(
	resource mcp.Resource,
	handler ResourceHandlerFunc,
) {
	if s.capabilities.resources == nil {
		panic("Resource capabilities not enabled")
	}
	s.resources[resource.URI] = resourceEntry{
		resource: resource,
		handler:  handler,
	}
}

// AddResourceTemplate registers a new resource template and its handler
func (s *MCPServer) AddResourceTemplate(
	template mcp.ResourceTemplate,
	handler ResourceTemplateHandlerFunc,
) {
	if s.capabilities.resources == nil {
		panic("Resource capabilities not enabled")
	}
	s.resourceTemplates[template.URITemplate] = resourceTemplateEntry{
		template: template,
		handler:  handler,
	}
}

// AddPrompt registers a new prompt handler with the given name
func (s *MCPServer) AddPrompt(prompt mcp.Prompt, handler PromptHandlerFunc) {
	if s.capabilities.prompts == nil {
		panic("Prompt capabilities not enabled")
	}
	s.prompts[prompt.Name] = prompt
	s.promptHandlers[prompt.Name] = handler
}

// AddTool registers a new tool and its handler
func (s *MCPServer) AddTool(tool mcp.Tool, handler ToolHandlerFunc) {
	s.tools[tool.Name] = tool
	s.toolHandlers[tool.Name] = handler
}

// AddNotificationHandler registers a new handler for incoming notifications
func (s *MCPServer) AddNotificationHandler(
	handler NotificationHandlerFunc,
) {
	s.notifications = append(s.notifications, handler)
}

func (s *MCPServer) handleInitialize(
	id interface{},
	request mcp.InitializeRequest,
) mcp.JSONRPCMessage {
	capabilities := mcp.ServerCapabilities{}

	if s.capabilities.resources != nil {
		capabilities.Resources = &struct {
			Subscribe   bool `json:"subscribe,omitempty"`
			ListChanged bool `json:"listChanged,omitempty"`
		}{
			Subscribe:   s.capabilities.resources.subscribe,
			ListChanged: s.capabilities.resources.listChanged,
		}
	}

	if s.capabilities.prompts != nil {
		capabilities.Prompts = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{
			ListChanged: s.capabilities.prompts.listChanged,
		}
	}

	// Only include Tools capability if there are registered tools
	if len(s.tools) > 0 {
		capabilities.Tools = &struct {
			ListChanged bool `json:"listChanged,omitempty"`
		}{
			ListChanged: true, // Always true when tools are present
		}
	}

	if s.capabilities.logging {
		capabilities.Logging = &struct{}{}
	}

	result := mcp.InitializeResult{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ServerInfo: mcp.Implementation{
			Name:    s.name,
			Version: s.version,
		},
		Capabilities: capabilities,
	}

	return createResponse(id, result)
}

func (s *MCPServer) handlePing(
	id interface{},
	request mcp.PingRequest,
) mcp.JSONRPCMessage {
	return createResponse(id, mcp.EmptyResult{})
}

func (s *MCPServer) handleListResources(
	id interface{},
	request mcp.ListResourcesRequest,
) mcp.JSONRPCMessage {
	resources := make([]mcp.Resource, 0, len(s.resources))
	for _, entry := range s.resources {
		resources = append(resources, entry.resource)
	}

	result := mcp.ListResourcesResult{
		Resources: resources,
	}
	if request.Params.Cursor != "" {
		result.NextCursor = "" // Handle pagination if needed
	}
	return createResponse(id, result)
}

func (s *MCPServer) handleListResourceTemplates(
	id interface{},
	request mcp.ListResourceTemplatesRequest,
) mcp.JSONRPCMessage {
	templates := make([]mcp.ResourceTemplate, 0, len(s.resourceTemplates))
	for _, entry := range s.resourceTemplates {
		templates = append(templates, entry.template)
	}

	result := mcp.ListResourceTemplatesResult{
		ResourceTemplates: templates,
	}
	if request.Params.Cursor != "" {
		result.NextCursor = "" // Handle pagination if needed
	}
	return createResponse(id, result)
}

func (s *MCPServer) handleReadResource(
	id interface{},
	request mcp.ReadResourceRequest,
) mcp.JSONRPCMessage {
	// First try direct resource handlers
	if entry, ok := s.resources[request.Params.URI]; ok {
		contents, err := entry.handler(request)
		if err != nil {
			return createErrorResponse(id, mcp.INTERNAL_ERROR, err.Error())
		}
		return createResponse(id, mcp.ReadResourceResult{Contents: contents})
	}

	// If no direct handler found, try matching against templates
	for uriTemplate, entry := range s.resourceTemplates {
		if matchesTemplate(request.Params.URI, uriTemplate) {
			contents, err := entry.handler(request)
			if err != nil {
				return createErrorResponse(id, mcp.INTERNAL_ERROR, err.Error())
			}
			return createResponse(
				id,
				mcp.ReadResourceResult{Contents: contents},
			)
		}
	}

	return createErrorResponse(
		id,
		mcp.INVALID_PARAMS,
		fmt.Sprintf(
			"No handler found for resource URI: %s",
			request.Params.URI,
		),
	)
}

// matchesTemplate checks if a URI matches a URI template pattern
func matchesTemplate(uri string, template string) bool {
	// Convert template into a regex pattern
	pattern := template
	// Replace {name} with ([^/]+)
	pattern = regexp.QuoteMeta(pattern)
	pattern = regexp.MustCompile(`\\\{[^}]+\\\}`).
		ReplaceAllString(pattern, `([^/]+)`)
	pattern = "^" + pattern + "$"

	matched, _ := regexp.MatchString(pattern, uri)
	return matched
}

func (s *MCPServer) handleListPrompts(
	id interface{},
	request mcp.ListPromptsRequest,
) mcp.JSONRPCMessage {
	prompts := make([]mcp.Prompt, 0, len(s.prompts))
	for _, prompt := range s.prompts {
		prompts = append(prompts, prompt)
	}

	result := mcp.ListPromptsResult{
		Prompts: prompts,
	}
	if request.Params.Cursor != "" {
		result.NextCursor = "" // Handle pagination if needed
	}
	return createResponse(id, result)
}

func (s *MCPServer) handleGetPrompt(
	id interface{},
	request mcp.GetPromptRequest,
) mcp.JSONRPCMessage {
	handler, ok := s.promptHandlers[request.Params.Name]
	if !ok {
		return createErrorResponse(
			id,
			mcp.INVALID_PARAMS,
			fmt.Sprintf("Prompt not found: %s", request.Params.Name),
		)
	}

	result, err := handler(request.Params.Arguments)
	if err != nil {
		return createErrorResponse(id, mcp.INTERNAL_ERROR, err.Error())
	}

	return createResponse(id, result)
}

func (s *MCPServer) handleListTools(
	id interface{},
	request mcp.ListToolsRequest,
) mcp.JSONRPCMessage {
	tools := make([]mcp.Tool, 0, len(s.tools))
	for name := range s.tools {
		tools = append(tools, s.tools[name])
	}

	result := mcp.ListToolsResult{
		Tools: tools,
	}
	if request.Params.Cursor != "" {
		result.NextCursor = "" // Handle pagination if needed
	}
	return createResponse(id, result)
}

func (s *MCPServer) handleToolCall(
	id interface{},
	request mcp.CallToolRequest,
) mcp.JSONRPCMessage {
	handler, ok := s.toolHandlers[request.Params.Name]
	if !ok {
		return createErrorResponse(
			id,
			mcp.INVALID_PARAMS,
			fmt.Sprintf("Tool not found: %s", request.Params.Name),
		)
	}

	result, err := handler(request.Params.Arguments)
	if err != nil {
		return createErrorResponse(id, mcp.INTERNAL_ERROR, err.Error())
	}

	return createResponse(id, result)
}

func (s *MCPServer) handleNotification(
	notification mcp.JSONRPCNotification,
) mcp.JSONRPCMessage {
	for _, handler := range s.notifications {
		handler(notification)
	}
	return nil
}

func createResponse(id interface{}, result interface{}) mcp.JSONRPCMessage {
	return mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      id,
		Result:  result,
	}
}

func createErrorResponse(
	id interface{},
	code int,
	message string,
) mcp.JSONRPCMessage {
	return mcp.JSONRPCError{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      id,
		Error: struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
		},
	}
}
