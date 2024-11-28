package server

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewDefaultServer(t *testing.T) {
	server := NewDefaultServer("test-server", "1.0.0")
	if server == nil {
		t.Fatal("Expected non-nil server")
	}

	// Check that all default handlers are registered
	methods := []string{
		"initialize",
		"ping",
		"resources/list",
		"resources/read",
		"resources/subscribe",
		"resources/unsubscribe",
		"prompts/list",
		"prompts/get",
		"tools/list",
		"tools/call",
		"logging/setLevel",
		"completion/complete",
	}

	for _, method := range methods {
		if _, ok := server.handlers[method]; !ok {
			t.Errorf("Expected handler for method %s to be registered", method)
		}
	}
}

func TestDefaultServer_Request(t *testing.T) {
	server := NewDefaultServer("test-server", "1.0.0")
	ctx := context.Background()

	tests := []struct {
		name       string
		method     string
		params     interface{}
		wantResult interface{}
		wantErr    bool
	}{
		{
			name:   "Initialize",
			method: "initialize",
			params: map[string]interface{}{
				"capabilities": mcp.ClientCapabilities{},
				"clientInfo": mcp.Implementation{
					Name:    "test-client",
					Version: "1.0",
				},
				"protocolVersion": "1.0",
			},
			wantResult: &mcp.InitializeResult{
				ServerInfo: mcp.Implementation{
					Name:    "test-server",
					Version: "1.0.0",
				},
				ProtocolVersion: "1.0",
				Capabilities: mcp.ServerCapabilities{
					Resources: &struct {
						ListChanged bool `json:"listChanged"`
						Subscribe   bool `json:"subscribe"`
					}{
						ListChanged: true,
						Subscribe:   true,
					},
				},
			},
			wantErr: false,
		},
		{
			name:       "Ping",
			method:     "ping",
			params:     struct{}{},
			wantResult: struct{}{},
			wantErr:    false,
		},
		{
			name:   "ListResources",
			method: "resources/list",
			params: map[string]interface{}{
				"cursor": nil,
			},
			wantResult: &mcp.ListResourcesResult{
				Resources: []mcp.Resource{},
			},
			wantErr: false,
		},
		{
			name:   "ReadResource",
			method: "resources/read",
			params: map[string]interface{}{
				"uri": "test://resource",
			},
			wantResult: &mcp.ReadResourceResult{
				Contents: []mcp.ResourceContents{},
			},
			wantErr: false,
		},
		{
			name:   "Subscribe",
			method: "resources/subscribe",
			params: map[string]interface{}{
				"uri": "test://resource",
			},
			wantResult: struct{}{},
			wantErr:    false,
		},
		{
			name:   "Unsubscribe",
			method: "resources/unsubscribe",
			params: map[string]interface{}{
				"uri": "test://resource",
			},
			wantResult: struct{}{},
			wantErr:    false,
		},
		{
			name:   "ListPrompts",
			method: "prompts/list",
			params: map[string]interface{}{
				"cursor": nil,
			},
			wantResult: &mcp.ListPromptsResult{
				Prompts: []mcp.Prompt{},
			},
			wantErr: false,
		},
		{
			name:   "GetPrompt",
			method: "prompts/get",
			params: map[string]interface{}{
				"name":      "test-prompt",
				"arguments": map[string]string{},
			},
			wantResult: &mcp.GetPromptResult{
				Messages: []mcp.PromptMessage{},
			},
			wantErr: false,
		},
		{
			name:   "ListTools",
			method: "tools/list",
			params: map[string]interface{}{
				"cursor": nil,
			},
			wantResult: &mcp.ListToolsResult{
				Tools: []mcp.Tool{},
			},
			wantErr: false,
		},
		{
			name:   "CallTool",
			method: "tools/call",
			params: map[string]interface{}{
				"name":      "test-tool",
				"arguments": map[string]interface{}{},
			},
			wantResult: &mcp.CallToolResult{
				Content: []mcp.Content{},
			},
			wantErr: false,
		},
		{
			name:   "SetLevel",
			method: "logging/setLevel",
			params: map[string]interface{}{
				"level": mcp.LoggingLevelInfo,
			},
			wantResult: struct{}{},
			wantErr:    false,
		},
		{
			name:   "Complete",
			method: "completion/complete",
			params: map[string]interface{}{
				"ref": map[string]interface{}{
					"type": "ref/prompt",
					"name": "test-prompt",
				},
				"argument": map[string]interface{}{
					"name":  "test-arg",
					"value": "test-value",
				},
			},
			wantResult: &mcp.CompleteResult{
				Completion: mcp.Completion{
					Values: []string{},
				},
			},
			wantErr: false,
		},
		{
			name:       "Invalid Method",
			method:     "invalid/method",
			params:     struct{}{},
			wantResult: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramsJSON, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("Failed to marshal params: %v", err)
			}

			gotResult, err := server.Request(ctx, tt.method, paramsJSON)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"DefaultServer.Request() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
				return
			}

			if !tt.wantErr {
				if !reflect.DeepEqual(gotResult, tt.wantResult) {
					t.Errorf(
						"DefaultServer.Request() = %v, want %v",
						gotResult,
						tt.wantResult,
					)
				}
			}
		})
	}
}

func TestDefaultServer_HandleOverride(t *testing.T) {
	server := NewDefaultServer("test-server", "1.0.0")
	ctx := context.Background()

	// Override ListResources handler
	customResource := mcp.Resource{
		Name: "custom-resource",
		URI:  "test://custom",
	}

	server.HandleListResources(
		func(ctx context.Context, cursor *string) (*mcp.ListResourcesResult, error) {
			return &mcp.ListResourcesResult{
				Resources: []mcp.Resource{customResource},
			}, nil
		},
	)

	// Test the overridden handler
	params := map[string]interface{}{
		"cursor": nil,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal params: %v", err)
	}

	result, err := server.Request(ctx, "resources/list", paramsJSON)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	listResult, ok := result.(*mcp.ListResourcesResult)
	if !ok {
		t.Fatal("Expected ListResourcesResult")
	}

	if len(listResult.Resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(listResult.Resources))
	}

	if !reflect.DeepEqual(listResult.Resources[0], customResource) {
		t.Errorf(
			"Got resource %v, want %v",
			listResult.Resources[0],
			customResource,
		)
	}
}

func TestDefaultServer_InvalidParams(t *testing.T) {
	server := NewDefaultServer("test-server", "1.0.0")
	ctx := context.Background()

	tests := []struct {
		name    string
		method  string
		params  string
		wantErr string
	}{
		{
			name:    "Invalid Initialize Params",
			method:  "initialize",
			params:  `{"invalid": "json"}`,
			wantErr: "missing required field",
		},
		{
			name:    "Invalid ListResources Params",
			method:  "resources/list",
			params:  `{"cursor": 123}`,
			wantErr: "json: cannot unmarshal number into Go struct field",
		},
		{
			name:    "Invalid JSON",
			method:  "ping",
			params:  `{ "stuff": "stuff" }`,
			wantErr: "ping method does not accept parameters",
		},
		{
			name:    "Missing Required URI",
			method:  "resources/read",
			params:  `{}`,
			wantErr: "uri is required",
		},
		{
			name:    "Invalid Level",
			method:  "logging/setLevel",
			params:  `{"level": "invalid"}`,
			wantErr: "invalid logging level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.Request(ctx, tt.method, json.RawMessage(tt.params))
			if err == nil {
				t.Error("Expected error for invalid params")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf(
					"Expected error containing %q, got %q",
					tt.wantErr,
					err.Error(),
				)
			}
		})
	}
}
