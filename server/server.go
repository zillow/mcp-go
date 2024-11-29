package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

type MCPServer interface {
	Request(
		ctx context.Context,
		method string,
		params json.RawMessage,
	) (interface{}, error)
	HandleInitialize(InitializeFunc)
	HandlePing(PingFunc)
	HandleListResources(ListResourcesFunc)
	HandleReadResource(ReadResourceFunc)
	HandleSubscribe(SubscribeFunc)
	HandleUnsubscribe(UnsubscribeFunc)
	HandleListPrompts(ListPromptsFunc)
	HandleGetPrompt(GetPromptFunc)
	HandleListTools(ListToolsFunc)
	HandleCallTool(CallToolFunc)
	HandleSetLevel(SetLevelFunc)
	HandleComplete(CompleteFunc)
	HandleNotification(string, NotificationFunc)
}

type InitializeFunc func(ctx context.Context, capabilities mcp.ClientCapabilities, clientInfo mcp.Implementation, protocolVersion string) (*mcp.InitializeResult, error)

type PingFunc func(ctx context.Context) error

type ListResourcesFunc func(ctx context.Context, cursor *string) (*mcp.ListResourcesResult, error)

type ReadResourceFunc func(ctx context.Context, uri string) (*mcp.ReadResourceResult, error)

type SubscribeFunc func(ctx context.Context, uri string) error

type UnsubscribeFunc func(ctx context.Context, uri string) error

type ListPromptsFunc func(ctx context.Context, cursor *string) (*mcp.ListPromptsResult, error)

type GetPromptFunc func(ctx context.Context, name string, arguments map[string]string) (*mcp.GetPromptResult, error)

type ListToolsFunc func(ctx context.Context, cursor *string) (*mcp.ListToolsResult, error)

type CallToolFunc func(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error)

type SetLevelFunc func(ctx context.Context, level mcp.LoggingLevel) error

type CompleteFunc func(ctx context.Context, ref interface{}, argument mcp.CompleteArgument) (*mcp.CompleteResult, error)

type NotificationFunc func(ctx context.Context, args any) (any, error)

type DefaultServer struct {
	handlers map[string]interface{}
	name     string
	version  string
}

// NewDefaultServer creates a new server with default handlers
func NewDefaultServer(name, version string) MCPServer {
	s := &DefaultServer{
		handlers: make(map[string]interface{}),
		name:     name,
		version:  version,
	}

	// Register default initialize handler
	s.HandleInitialize(s.defaultInitialize)

	// Register default handlers for other methods
	s.HandlePing(s.defaultPing)
	s.HandleListResources(s.defaultListResources)
	s.HandleReadResource(s.defaultReadResource)
	s.HandleSubscribe(s.defaultSubscribe)
	s.HandleUnsubscribe(s.defaultUnsubscribe)
	s.HandleListPrompts(s.defaultListPrompts)
	s.HandleGetPrompt(s.defaultGetPrompt)
	s.HandleListTools(s.defaultListTools)
	s.HandleCallTool(s.defaultCallTool)
	s.HandleSetLevel(s.defaultSetLevel)
	s.HandleComplete(s.defaultComplete)

	return s
}

// Request is the main entrypoint of the server
func (s *DefaultServer) Request(
	ctx context.Context,
	method string,
	params json.RawMessage,
) (interface{}, error) {

	// If params is nil, use empty object for methods that expect params
	if params == nil {
		params = json.RawMessage("{}")
	}

	// Handle notifications
	if strings.Contains(method, "notifications") {
		if s.handlers[method] == nil {
			return nil, nil
		}

		return s.handlers[method].(NotificationFunc)(ctx, params)
	}

	// Handle all other methods
	_, ok := s.handlers[method]
	if !ok {
		return nil, fmt.Errorf("method not found: %s", method)
	}

	switch method {
	case "initialize":
		var p struct {
			Capabilities    *mcp.ClientCapabilities `json:"capabilities"`
			ClientInfo      *mcp.Implementation     `json:"clientInfo"`
			ProtocolVersion string                  `json:"protocolVersion"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		if p.ClientInfo == nil {
			return nil, fmt.Errorf("missing required field: clientInfo")
		}
		if p.Capabilities == nil {
			return nil, fmt.Errorf("missing required field: capabilities")
		}
		if p.ProtocolVersion == "" {
			return nil, fmt.Errorf("missing required field: protocolVersion")
		}
		return s.handlers["initialize"].(InitializeFunc)(
			ctx,
			*p.Capabilities,
			*p.ClientInfo,
			p.ProtocolVersion,
		)

	case "ping":
		if len(params) > 0 && string(params) != "null" &&
			string(params) != "{}" {
			return nil, fmt.Errorf("ping method does not accept parameters")
		}
		return struct{}{}, s.handlers["ping"].(PingFunc)(ctx)

	case "resources/list":
		var p struct {
			Cursor *string `json:"cursor,omitempty"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		return s.handlers["resources/list"].(ListResourcesFunc)(ctx, p.Cursor)

	case "resources/read":
		var p struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		if p.URI == "" {
			return nil, fmt.Errorf("uri is required")
		}
		return s.handlers["resources/read"].(ReadResourceFunc)(ctx, p.URI)

	case "resources/subscribe":
		var p struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		if p.URI == "" {
			return nil, fmt.Errorf("uri is required")
		}
		err := s.handlers["resources/subscribe"].(SubscribeFunc)(ctx, p.URI)
		return struct{}{}, err

	case "resources/unsubscribe":
		var p struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		if p.URI == "" {
			return nil, fmt.Errorf("uri is required")
		}
		err := s.handlers["resources/unsubscribe"].(UnsubscribeFunc)(ctx, p.URI)
		return struct{}{}, err

	case "prompts/list":
		var p struct {
			Cursor *string `json:"cursor,omitempty"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		return s.handlers["prompts/list"].(ListPromptsFunc)(ctx, p.Cursor)

	case "prompts/get":
		var p struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		if p.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		return s.handlers["prompts/get"].(GetPromptFunc)(
			ctx,
			p.Name,
			p.Arguments,
		)

	case "tools/list":
		var p struct {
			Cursor *string `json:"cursor,omitempty"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		return s.handlers["tools/list"].(ListToolsFunc)(ctx, p.Cursor)

	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		if p.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		return s.handlers["tools/call"].(CallToolFunc)(ctx, p.Name, p.Arguments)

	case "logging/setLevel":
		var p struct {
			Level mcp.LoggingLevel `json:"level"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		// Validate logging level
		valid := false
		for _, l := range []mcp.LoggingLevel{
			mcp.LoggingLevelEmergency,
			mcp.LoggingLevelAlert,
			mcp.LoggingLevelCritical,
			mcp.LoggingLevelError,
			mcp.LoggingLevelWarning,
			mcp.LoggingLevelNotice,
			mcp.LoggingLevelInfo,
			mcp.LoggingLevelDebug,
		} {
			if p.Level == l {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("invalid logging level: %s", p.Level)
		}
		err := s.handlers["logging/setLevel"].(SetLevelFunc)(ctx, p.Level)
		return struct{}{}, err

	case "completion/complete":
		var p struct {
			Ref      interface{}          `json:"ref"`
			Argument mcp.CompleteArgument `json:"argument"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("failed to parse parameters: %w", err)
		}
		if p.Ref == nil {
			return nil, fmt.Errorf("ref is required")
		}
		if p.Argument.Name == "" {
			return nil, fmt.Errorf("argument name is required")
		}
		return s.handlers["completion/complete"].(CompleteFunc)(
			ctx,
			p.Ref,
			p.Argument,
		)
	}

	return nil, fmt.Errorf("method handler not implemented: %s", method)
}

// Handler registration methods
func (s *DefaultServer) HandleInitialize(
	f InitializeFunc,
) {
	s.handlers["initialize"] = f
}

func (s *DefaultServer) HandlePing(
	f PingFunc,
) {
	s.handlers["ping"] = f
}

func (s *DefaultServer) HandleListResources(
	f ListResourcesFunc,
) {
	s.handlers["resources/list"] = f
}

func (s *DefaultServer) HandleReadResource(
	f ReadResourceFunc,
) {
	s.handlers["resources/read"] = f
}

func (s *DefaultServer) HandleSubscribe(
	f SubscribeFunc,
) {
	s.handlers["resources/subscribe"] = f
}

func (s *DefaultServer) HandleUnsubscribe(
	f UnsubscribeFunc,
) {
	s.handlers["resources/unsubscribe"] = f
}

func (s *DefaultServer) HandleListPrompts(
	f ListPromptsFunc,
) {
	s.handlers["prompts/list"] = f
}

func (s *DefaultServer) HandleGetPrompt(
	f GetPromptFunc,
) {
	s.handlers["prompts/get"] = f
}

func (s *DefaultServer) HandleListTools(
	f ListToolsFunc,
) {
	s.handlers["tools/list"] = f
}

func (s *DefaultServer) HandleCallTool(
	f CallToolFunc,
) {
	s.handlers["tools/call"] = f
}

func (s *DefaultServer) HandleSetLevel(
	f SetLevelFunc,
) {
	s.handlers["logging/setLevel"] = f
}

func (s *DefaultServer) HandleComplete(
	f CompleteFunc,
) {
	s.handlers["completion/complete"] = f
}

func (s *DefaultServer) HandleNotification(name string, f NotificationFunc) {
	s.handlers["notifications/"+name] = f
}

// Default handlers
func (s *DefaultServer) defaultInitialize(
	ctx context.Context,
	capabilities mcp.ClientCapabilities,
	clientInfo mcp.Implementation,
	protocolVersion string,
) (*mcp.InitializeResult, error) {
	return &mcp.InitializeResult{
		ServerInfo: mcp.Implementation{
			Name:    s.name,
			Version: s.version,
		},
		ProtocolVersion: "2024-11-05",
		Capabilities: mcp.ServerCapabilities{
			Resources: &struct {
				ListChanged bool `json:"listChanged"`
				Subscribe   bool `json:"subscribe"`
			}{
				ListChanged: true,
				Subscribe:   true,
			},
		},
	}, nil
}

func (s *DefaultServer) defaultPing(ctx context.Context) error {
	return nil
}

func (s *DefaultServer) defaultListResources(
	ctx context.Context,
	cursor *string,
) (*mcp.ListResourcesResult, error) {
	return &mcp.ListResourcesResult{
		Resources: []mcp.Resource{},
	}, nil
}

func (s *DefaultServer) defaultReadResource(
	ctx context.Context,
	uri string,
) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{},
	}, nil
}

func (s *DefaultServer) defaultSubscribe(
	ctx context.Context,
	uri string,
) error {
	return nil
}

func (s *DefaultServer) defaultUnsubscribe(
	ctx context.Context,
	uri string,
) error {
	return nil
}

func (s *DefaultServer) defaultListPrompts(
	ctx context.Context,
	cursor *string,
) (*mcp.ListPromptsResult, error) {
	return &mcp.ListPromptsResult{
		Prompts: []mcp.Prompt{},
	}, nil
}

func (s *DefaultServer) defaultGetPrompt(
	ctx context.Context,
	name string,
	arguments map[string]string,
) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Messages: []mcp.PromptMessage{},
	}, nil
}

func (s *DefaultServer) defaultListTools(
	ctx context.Context,
	cursor *string,
) (*mcp.ListToolsResult, error) {
	return &mcp.ListToolsResult{
		Tools: []mcp.Tool{},
	}, nil
}

func (s *DefaultServer) defaultCallTool(
	ctx context.Context,
	name string,
	arguments map[string]interface{},
) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{},
	}, nil
}

func (s *DefaultServer) defaultSetLevel(
	ctx context.Context,
	level mcp.LoggingLevel,
) error {
	return nil
}

func (s *DefaultServer) defaultComplete(
	ctx context.Context,
	ref interface{},
	argument mcp.CompleteArgument,
) (*mcp.CompleteResult, error) {
	return &mcp.CompleteResult{
		Completion: mcp.Completion{
			Values: []string{},
		},
	}, nil
}
