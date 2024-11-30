package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestNewDefaultServer(t *testing.T) {
	s := NewDefaultServer("test", "1.0.0")
	assert.NotNil(t, s)
	assert.IsType(t, &DefaultServer{}, s)
}

func TestDefaultServer_Request(t *testing.T) {
	s := NewDefaultServer("test", "1.0.0")
	ctx := context.Background()

	tests := []struct {
		name           string
		request        JSONRPCRequest
		expectedResult interface{}
		expectedError  *JSONRPCResponse
	}{
		{
			name: "Initialize",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params: json.RawMessage(
					`{"capabilities":{},"clientInfo":{"name":"test",
  "version":"1.0.0"},"protocolVersion":"2024-11-05"}`,
				),
			},
			expectedResult: &mcp.InitializeResult{},
		},
		{
			name: "Ping",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      2,
				Method:  "ping",
				Params:  json.RawMessage(`{}`),
			},
			expectedResult: struct{}{},
		},
		{
			name: "ListResources",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "resources/list",
				Params:  json.RawMessage(`{}`),
			},
			expectedResult: &mcp.ListResourcesResult{},
		},
		{
			name: "ReadResource",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      4,
				Method:  "resources/read",
				Params:  json.RawMessage(`{"uri":"test"}`),
			},
			expectedResult: &mcp.ReadResourceResult{},
		},
		{
			name: "InvalidMethod",
			request: JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      5,
				Method:  "invalid",
				Params:  json.RawMessage(`{}`),
			},
			expectedError: &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      5,
				Error: &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    -32601,
					Message: "method not found: invalid",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Request(ctx, tt.request)

			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, "2.0", result.JSONRPC)
				assert.Equal(t, tt.request.ID, result.ID)
				assert.Nil(t, result.Error)
				assert.IsType(t, tt.expectedResult, result.Result)
			}
		})
	}
}

func TestDefaultServer_HandleNotification(t *testing.T) {
	s := NewDefaultServer("test", "1.0.0")
	ctx := context.Background()

	s.HandleNotification(
		"test",
		func(ctx context.Context, args any) (any, error) {
			return "notification handled", nil
		},
	)

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/test",
		Params:  json.RawMessage(`{}`),
	}

	result := s.Request(ctx, request)
	assert.NotNil(t, result)
	assert.Equal(t, "2.0", result.JSONRPC)
	assert.Nil(t, result.Error)
	assert.Equal(t, "notification handled", result.Result)
}

func TestDefaultServer_HandlersRegistration(t *testing.T) {
	s := NewDefaultServer("test", "1.0.0")

	handlers := []struct {
		name string
		fn   interface{}
	}{
		{"Initialize", func(InitializeFunc) {}},
		{"Ping", func(PingFunc) {}},
		{"ListResources", func(ListResourcesFunc) {}},
		{"ReadResource", func(ReadResourceFunc) {}},
		{"Subscribe", func(SubscribeFunc) {}},
		{"Unsubscribe", func(UnsubscribeFunc) {}},
		{"ListPrompts", func(ListPromptsFunc) {}},
		{"GetPrompt", func(GetPromptFunc) {}},
		{"ListTools", func(ListToolsFunc) {}},
		{"CallTool", func(CallToolFunc) {}},
		{"SetLevel", func(SetLevelFunc) {}},
		{"Complete", func(CompleteFunc) {}},
	}

	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				s.(*DefaultServer).handlers[getMethodName(h.name)] = h.fn
			})
		})
	}
}

func getMethodName(handlerName string) string {
	switch handlerName {
	case "Initialize":
		return "initialize"
	case "Ping":
		return "ping"
	case "ListResources":
		return "resources/list"
	case "ReadResource":
		return "resources/read"
	case "Subscribe":
		return "resources/subscribe"
	case "Unsubscribe":
		return "resources/unsubscribe"
	case "ListPrompts":
		return "prompts/list"
	case "GetPrompt":
		return "prompts/get"
	case "ListTools":
		return "tools/list"
	case "CallTool":
		return "tools/call"
	case "SetLevel":
		return "logging/setLevel"
	case "Complete":
		return "completion/complete"
	default:
		return ""
	}
}
