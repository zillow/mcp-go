package client

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSEMCPClient(t *testing.T) {
	// Create context with timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create default server and test server
	mcpServer := server.NewDefaultServer("test-server", "1.0.0")
	_, testServer := server.NewTestServer(mcpServer)

	// Ensure test server is closed
	t.Cleanup(func() {
		testServer.Close()
	})

	// Create SSE client
	client, err := NewSSEMCPClient(testServer.URL + "/sse")
	require.NoError(t, err)

	// Start client and ensure it's closed
	err = client.Start(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := client.Close()
		if err != nil {
			t.Logf("Error closing client: %v", err)
		}
	})

	// Wait for endpoint to be received
	err = waitForEndpoint(client, 2*time.Second)
	require.NoError(t, err, "Failed to receive endpoint")

	t.Run("Initialize", func(t *testing.T) {
		result, err := client.Initialize(
			ctx,
			mcp.ClientCapabilities{},
			mcp.Implementation{
				Name: "test-client",

				Version: "1.0.0",
			},
			"2024-11-05",
		)

		assert.NoError(t, err)
		assert.Equal(t, "test-server", result.ServerInfo.Name)
		assert.Equal(t, "1.0.0", result.ServerInfo.Version)
		assert.Equal(t, "2024-11-05", result.ProtocolVersion)
		assert.True(t, result.Capabilities.Resources.ListChanged)
		assert.True(t, result.Capabilities.Resources.Subscribe)
	})

	t.Run("Ping", func(t *testing.T) {
		err := client.Ping(ctx)
		assert.NoError(t, err)
	})

	t.Run("ListResources", func(t *testing.T) {
		result, err := client.ListResources(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Resources)
	})

	t.Run("ReadResource", func(t *testing.T) {
		result, err := client.ReadResource(ctx, "test://resource1")
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Contents)
	})

	t.Run("Subscribe and Unsubscribe", func(t *testing.T) {
		err := client.Subscribe(ctx, "test://resource1")
		assert.NoError(t, err)

		err = client.Unsubscribe(ctx, "test://resource1")
		assert.NoError(t, err)
	})

	t.Run("ListPrompts", func(t *testing.T) {
		result, err := client.ListPrompts(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Prompts)
	})

	t.Run("GetPrompt", func(t *testing.T) {
		result, err := client.GetPrompt(ctx, "test-prompt", nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Messages)
	})

	t.Run("ListTools", func(t *testing.T) {
		result, err := client.ListTools(ctx, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Tools)
	})

	t.Run("CallTool", func(t *testing.T) {
		result, err := client.CallTool(ctx, "test-tool", nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Content)
	})

	t.Run("SetLevel", func(t *testing.T) {
		err := client.SetLevel(ctx, mcp.LoggingLevelDebug)
		assert.NoError(t, err)
	})

	t.Run("Complete", func(t *testing.T) {
		result, err := client.Complete(ctx, "test-ref", mcp.CompleteArgument{
			Name: "test-arg",
		})
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result.Completion.Values)
	})
}

func TestSSEMCPClientErrors(t *testing.T) {
	// Create context with timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test with invalid URL
	_, err := NewSSEMCPClient("://invalid-url")
	assert.Error(t, err)

	// Test methods before initialization
	client, err := NewSSEMCPClient("http://localhost:8080/sse")
	require.NoError(t, err)

	_, err = client.ListResources(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client not initialized")

	// Test with invalid method parameters
	mcpServer := server.NewDefaultServer("test-server", "1.0.0")
	_, testServer := server.NewTestServer(mcpServer)

	// Ensure test server is closed
	t.Cleanup(func() {
		testServer.Close()
	})

	client, err = NewSSEMCPClient(testServer.URL + "/sse")
	require.NoError(t, err)

	err = client.Start(ctx)
	require.NoError(t, err)

	// Ensure client is closed
	t.Cleanup(func() {
		err := client.Close()
		if err != nil {
			t.Logf("Error closing client: %v", err)
		}
	})

	// Wait for endpoint to be received
	err = waitForEndpoint(client, 2*time.Second)
	require.NoError(t, err, "Failed to receive endpoint")

	// Test initialize with missing required fields
	_, err = client.Initialize(
		ctx,
		mcp.ClientCapabilities{},
		mcp.Implementation{},
		"",
	)
	assert.Error(t, err)
}

func waitForEndpoint(client *SSEMCPClient, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if client.GetEndpoint() != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for endpoint")
}
