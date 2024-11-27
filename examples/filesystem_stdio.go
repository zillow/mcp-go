package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/mark3labs/mcp-go/client"
)

// Content types
type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeImage ContentType = "image"
)

// TextContent represents text content in a message
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallToolResult represents the result of a tool call
type CallToolResult struct {
	Content []json.RawMessage `json:"content"`
	IsError bool              `json:"isError,omitempty"`
}

// FilesystemClient provides a high-level interface to the Filesystem MCP server
type FilesystemClient struct {
	transport *client.StdioTransport
}

// NewFilesystemClient creates a new client connected to the Filesystem MCP server
func NewFilesystemClient() (*FilesystemClient, error) {
	// Create transport with /tmp as the allowed directory
	transport := client.NewStdioTransport(
		"/etc/profiles/per-user/space_cowboy/bin/npx",
		[]string{
			"-y",
			"@modelcontextprotocol/server-filesystem",
			"/tmp",
		},
		client.WithStdioDir("/tmp"),
	)

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &FilesystemClient{transport: transport}, nil
}

// ListDirectory lists the contents of a directory
func (fc *FilesystemClient) ListDirectory(
	ctx context.Context,
	path string,
) ([]string, error) {
	result, err := fc.callTool(ctx, "list_directory", map[string]interface{}{
		"path": path,
	})
	if err != nil {
		return nil, err
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("no content returned")
	}

	// Parse the content
	var textContent TextContent
	if err := json.Unmarshal(result.Content[0], &textContent); err != nil {
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}

	// Split the text into lines
	entries := strings.Split(strings.TrimSpace(textContent.Text), "\n")
	return entries, nil
}

// CreateDirectory creates a new directory
func (fc *FilesystemClient) CreateDirectory(
	ctx context.Context,
	path string,
) error {
	_, err := fc.callTool(ctx, "create_directory", map[string]interface{}{
		"path": path,
	})
	return err
}

// WriteFile writes content to a file
func (fc *FilesystemClient) WriteFile(
	ctx context.Context,
	path, content string,
) error {
	_, err := fc.callTool(ctx, "write_file", map[string]interface{}{
		"path":    path,
		"content": content,
	})
	return err
}

// helper function to call tools
func (fc *FilesystemClient) callTool(
	ctx context.Context,
	name string,
	args map[string]interface{},
) (*CallToolResult, error) {
	msg := &client.JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
		ID: 1,
	}

	err := fc.transport.Send(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	response, err := fc.transport.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}

	if response.Error != nil {
		return nil, fmt.Errorf("server error: %s", response.Error.Message)
	}

	// Marshal the result back to JSON
	resultBytes, err := json.Marshal(response.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var result CallToolResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}

	if result.IsError {
		return nil, fmt.Errorf("tool execution failed")
	}

	return &result, nil
}

func main() {
	// Create a new client
	client, err := NewFilesystemClient()
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()

	// List directory contents
	fmt.Println("Listing /tmp directory:")
	entries, err := client.ListDirectory(ctx, "/tmp")
	if err != nil {
		log.Fatalf("Failed to list directory: %v", err)
	}
	for _, entry := range entries {
		fmt.Println(entry)
	}

	// Create mcp directory
	fmt.Println("\nCreating /tmp/mcp directory...")
	err = client.CreateDirectory(ctx, "/tmp/mcp")
	if err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	// Create and write to file
	fmt.Println("Creating and writing to /tmp/mcp/test.txt...")
	err = client.WriteFile(ctx, "/tmp/mcp/test.txt", "hello world")
	if err != nil {
		log.Fatalf("Failed to write file: %v", err)
	}

	// List directory again to show changes
	fmt.Println("\nListing /tmp/mcp directory:")
	entries, err = client.ListDirectory(ctx, "/tmp/mcp")
	if err != nil {
		log.Fatalf("Failed to list directory: %v", err)
	}
	for _, entry := range entries {
		fmt.Println(entry)
	}
}
