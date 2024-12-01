package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolName string

const (
	ECHO                   ToolName = "echo"
	ADD                    ToolName = "add"
	LONG_RUNNING_OPERATION ToolName = "longRunningOperation"
	SAMPLE_LLM             ToolName = "sampleLLM"
	GET_TINY_IMAGE         ToolName = "getTinyImage"
)

type PromptName string

const (
	SIMPLE  PromptName = "simple_prompt"
	COMPLEX PromptName = "complex_prompt"
)

type MCPServer struct {
	server        server.MCPServer
	subscriptions map[string]bool
	updateTicker  *time.Ticker
	allResources  []mcp.Resource
}

func NewMCPServer() *MCPServer {
	s := &MCPServer{
		server: server.NewDefaultServer(
			"example-servers/everything",
			"1.0.0",
		),
		subscriptions: make(map[string]bool),
		updateTicker:  time.NewTicker(5 * time.Second),
		allResources:  generateResources(),
	}

	s.server.HandleInitialize(s.handleInitialize)
	s.server.HandleListResources(s.handleListResources)
	s.server.HandleListResourceTemplates(s.handleListResourceTemplates)
	s.server.HandleReadResource(s.handleReadResource)
	s.server.HandleSubscribe(s.handleSubscribe)
	s.server.HandleUnsubscribe(s.handleUnsubscribe)
	s.server.HandleListPrompts(s.handleListPrompts)
	s.server.HandleGetPrompt(s.handleGetPrompt)
	s.server.HandleListTools(s.handleListTools)
	s.server.HandleCallTool(s.handleCallTool)
	s.server.HandleSetLevel(s.handleSetLevel)

	go s.runUpdateInterval()

	return s
}

func generateResources() []mcp.Resource {
	resources := make([]mcp.Resource, 100)
	for i := 0; i < 100; i++ {
		uri := fmt.Sprintf("test://static/resource/%d", i+1)
		if i%2 == 0 {
			resources[i] = mcp.Resource{
				Uri:      uri,
				Name:     fmt.Sprintf("Resource %d", i+1),
				MimeType: "text/plain",
				Text: fmt.Sprintf(
					"Resource %d: This is a plaintext resource",
					i+1,
				),
			}
		} else {
			resources[i] = mcp.Resource{
				Uri:      uri,
				Name:     fmt.Sprintf("Resource %d", i+1),
				MimeType: "application/octet-stream",
				Blob:     base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("Resource %d: This is a base64 blob", i+1))),
			}
		}
	}
	return resources
}

func (s *MCPServer) runUpdateInterval() {
	for range s.updateTicker.C {
		for uri := range s.subscriptions {
			s.server.HandleNotification(
				"resources/updated",
				func(ctx context.Context,
					args any) (any, error) {
					return map[string]string{"uri": uri}, nil
				},
			)
		}
	}
}

func (s *MCPServer) handleInitialize(
	ctx context.Context,
	capabilities mcp.
		ClientCapabilities,
	clientInfo mcp.Implementation,
	protocolVersion string,
) (*mcp.
	InitializeResult, error) {
	return &mcp.InitializeResult{
		ServerInfo: mcp.Implementation{
			Name:    "example-servers/everything",
			Version: "1.0.0",
		},
		ProtocolVersion: "2024-11-05",
		Capabilities: mcp.ServerCapabilities{
			Prompts:   &mcp.ServerCapabilitiesPrompts{},
			Resources: &mcp.ServerCapabilitiesResources{Subscribe: true},
			Tools:     &mcp.ServerCapabilitiesTools{},
			Logging:   mcp.ServerCapabilitiesLogging{},
		},
	}, nil
}

func (s *MCPServer) handleListResources(
	ctx context.Context,
	cursor *string,
) (*mcp.ListResourcesResult, error) {
	const pageSize = 10
	startIndex := 0

	if cursor != nil {
		decodedCursor, err := base64.StdEncoding.DecodeString(*cursor)
		if err == nil {
			startIndex, _ = strconv.Atoi(string(decodedCursor))
		}
	}

	endIndex := startIndex + pageSize
	if endIndex > len(s.allResources) {
		endIndex = len(s.allResources)
	}

	resources := make([]mcp.Resource, endIndex-startIndex)
	for i, r := range s.allResources[startIndex:endIndex] {
		resources[i] = mcp.Resource{
			Uri:      r.Uri,
			Name:     r.Name,
			MimeType: r.MimeType,
			Text:     r.Text,
			Blob:     r.Blob,
		}
	}

	var nextCursor *string
	if endIndex < len(s.allResources) {
		encodedCursor := base64.StdEncoding.EncodeToString([]byte(strconv.
			Itoa(endIndex)))
		nextCursor = &encodedCursor
	}

	return &mcp.ListResourcesResult{
		Resources:  resources,
		NextCursor: *nextCursor,
	}, nil
}

func (s *MCPServer) handleListResourceTemplates(
	ctx context.Context,
	cursor *string,
) (*mcp.ListResourceTemplatesResult, error) {
	return &mcp.ListResourceTemplatesResult{
		ResourceTemplates: []mcp.ResourceTemplate{
			{
				UriTemplate: "test://static/resource/{id}",
				Name:        "Static Resource",
				Description: "A static resource with a numeric ID",
			},
		},
	}, nil
}

func (s *MCPServer) handleReadResource(ctx context.Context, uri string) (*mcp.
	ReadResourceResult, error) {
	if strings.HasPrefix(uri, "test://static/resource/") {
		parts := strings.Split(uri, "/")
		if len(parts) > 0 {
			index, err := strconv.Atoi(parts[len(parts)-1])
			if err == nil && index > 0 && index <= len(s.allResources) {
				resource := s.allResources[index-1]
				return &mcp.ReadResourceResult{
					Contents: []interface{}{resource},
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("unknown resource: %s", uri)
}

func (s *MCPServer) handleSubscribe(ctx context.Context, uri string) error {
	s.subscriptions[uri] = true
	// Implement requestSampling here if needed
	return nil
}

func (s *MCPServer) handleUnsubscribe(ctx context.Context, uri string) error {
	delete(s.subscriptions, uri)
	return nil
}

func (s *MCPServer) handleListPrompts(
	ctx context.Context,
	cursor *string,
) (*mcp.ListPromptsResult, error) {
	return &mcp.ListPromptsResult{
		Prompts: []mcp.Prompt{
			{
				Name:        string(SIMPLE),
				Description: "A prompt without arguments",
			},
			{
				Name:        string(COMPLEX),
				Description: "A prompt with arguments",
				Arguments: []mcp.PromptArgument{
					{
						Name:        "temperature",
						Description: "Temperature setting",
						Required:    true,
					},
					{
						Name:        "style",
						Description: "Output style",
						Required:    false,
					},
				},
			},
		},
	}, nil
}

func (s *MCPServer) handleGetPrompt(ctx context.Context, name string,
	arguments map[string]string) (*mcp.GetPromptResult, error) {
	switch PromptName(name) {
	case SIMPLE:
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: "user",
					Content: mcp.TextContent{
						Type: "text",
						Text: "This is a simple prompt without arguments.",
					},
				},
			},
		}, nil
	case COMPLEX:
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				{
					Role: "user",
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf(
							"This is a complex prompt with arguments: temperature=%s, style=%s",
							arguments["temperature"],
							arguments["style"],
						),
					},
				},
				{
					Role: "assistant",
					Content: mcp.TextContent{
						Type: "text",
						Text: "I understand. You've provided a complex prompt with temperature and style arguments. How would you like me to proceed?",
					},
				},
				{
					Role: "user",
					Content: mcp.ImageContent{
						Type:     "image",
						Data:     MCP_TINY_IMAGE,
						MimeType: "image/png",
					},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown prompt: %s", name)
	}
}

func (s *MCPServer) handleListTools(ctx context.Context, cursor *string) (*mcp.
	ListToolsResult, error) {
	tools := []mcp.Tool{
		{
			Name:        string(ECHO),
			Description: "Echoes back the input",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: mcp.ToolInputSchemaProperties{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "Message to echo",
					},
				},
			},
		},
		{
			Name:        string(ADD),
			Description: "Adds two numbers",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: mcp.ToolInputSchemaProperties{
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
			Name:        string(LONG_RUNNING_OPERATION),
			Description: "Demonstrates a long running operation with progress updates",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: mcp.ToolInputSchemaProperties{
					"duration": map[string]interface{}{
						"type":        "number",
						"description": "Duration of the operation in seconds",
						"default":     10,
					},
					"steps": map[string]interface{}{
						"type":        "number",
						"description": "Number of steps in the operation",
						"default":     5,
					},
				},
			},
		},
		{
			Name:        string(SAMPLE_LLM),
			Description: "Samples from an LLM using MCP's sampling feature",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: mcp.ToolInputSchemaProperties{
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "The prompt to send to the LLM",
					},
					"maxTokens": map[string]interface{}{
						"type":        "number",
						"description": "Maximum number of tokens to generate",
						"default":     100,
					},
				},
			},
		},
		{
			Name:        string(GET_TINY_IMAGE),
			Description: "Returns the MCP_TINY_IMAGE",
			InputSchema: mcp.ToolInputSchema{
				Type:       "object",
				Properties: mcp.ToolInputSchemaProperties{},
			},
		},
	}

	return &mcp.ListToolsResult{Tools: tools}, nil
}

func (s *MCPServer) handleCallTool(
	ctx context.Context,
	name string,
	arguments map[string]interface{},
) (*mcp.CallToolResult, error) {
	switch ToolName(name) {
	case ECHO:
		message, ok := arguments["message"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid message argument")
		}
		return &mcp.CallToolResult{
			Content: []interface{}{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Echo: %s", message),
				},
			},
		}, nil

	case ADD:
		a, ok1 := arguments["a"].(float64)
		b, ok2 := arguments["b"].(float64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("invalid number arguments")
		}
		sum := a + b
		return &mcp.CallToolResult{
			Content: []interface{}{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("The sum of %f and %f is %f.", a, b, sum),
				},
			},
		}, nil

	case LONG_RUNNING_OPERATION:
		duration, _ := arguments["duration"].(float64)
		steps, _ := arguments["steps"].(float64)
		stepDuration := duration / steps
		progressToken, _ := arguments["_meta"].(map[string]interface{})["progressToken"].(string)

		for i := 1; i < int(steps)+1; i++ {
			time.Sleep(time.Duration(stepDuration * float64(time.Second)))
			if progressToken != "" {
				s.server.HandleNotification(
					"progress",
					func(ctx context.Context, args any) (any, error) {
						return map[string]interface{}{
							"progress":      i,
							"total":         int(steps),
							"progressToken": progressToken,
						}, nil
					},
				)
			}
		}

		return &mcp.CallToolResult{
			Content: []interface{}{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf(
						"Long running operation completed. Duration: %f seconds, Steps: %d.",
						duration,
						int(steps),
					),
				},
			},
		}, nil

	// TODO Implement
	// case SAMPLE_LLM:
	// 	prompt, _ := arguments["prompt"].(string)
	// 	maxTokens, _ := arguments["maxTokens"].(float64)
	// 	// Implement requestSampling here
	// 	result := "Sample LLM result" // Replace with actual implementation
	// 	return &mcp.CallToolResult{
	// 		Content: []interface{}{
	// 			mcp.TextContent{
	// 				Type: "text",
	// 				Text: fmt.Sprintf("LLM sampling result: %s", result),
	// 			},
	// 		},
	// 	}, nil

	case GET_TINY_IMAGE:
		return &mcp.CallToolResult{
			Content: []interface{}{
				mcp.TextContent{
					Type: "text",
					Text: "This is a tiny image:",
				},
				mcp.ImageContent{
					Type:     "image",
					Data:     MCP_TINY_IMAGE,
					MimeType: "image/png",
				},
				mcp.TextContent{
					Type: "text",
					Text: "The image above is the MCP tiny image.",
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *MCPServer) handleSetLevel(ctx context.Context, level mcp.
	LoggingLevel) error {
	s.server.HandleNotification(
		"message",
		func(ctx context.Context, args any) (any, error) {
			return map[string]interface{}{
				"level":  "debug",
				"logger": "test-server",
				"data":   fmt.Sprintf("Logging level set to: %s", level),
			}, nil
		},
	)
	return nil
}

func (s *MCPServer) Serve() error {
	return server.ServeStdio(s.server)
}

func main() {
	server := NewMCPServer()
	if err := server.Serve(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

const MCP_TINY_IMAGE = "..."
