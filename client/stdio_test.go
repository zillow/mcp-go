package client

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func compileTestServer(outputPath string) error {
	cwd, _ := os.Getwd()
	cmd := exec.Command(
		"go",
		"build",
		"-o",
		outputPath,
		filepath.Join(cwd, "..", "testdata", "mockstdio_server.go"),
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compilation failed: %v\nOutput: %s", err, output)
	}
	return nil
}

func TestStdioMCPClient(t *testing.T) {
	// Compile mock server
	mockServerPath := filepath.Join("testdata", "mockstdio_server")
	if err := compileTestServer(mockServerPath); err != nil {
		t.Fatalf("Failed to compile mock server: %v", err)
	}
	defer os.Remove(mockServerPath)

	client, err := NewStdioMCPClient(mockServerPath)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	t.Run("Initialize", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		capabilities := ClientCapabilities{
			Experimental: map[string]map[string]interface{}{},
			Roots: &struct {
				ListChanged bool `json:"listChanged"`
			}{
				ListChanged: true,
			},
		}

		clientInfo := Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		}

		result, err := client.Initialize(ctx, capabilities, clientInfo, "1.0")
		if err != nil {
			t.Fatalf("Initialize failed: %v", err)
		}

		if result.ProtocolVersion == "" {
			t.Error("Expected protocol version in response")
		}

		if result.ServerInfo.Name == "" {
			t.Error("Expected server info in response")
		}
	})

	t.Run("Ping", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.Ping(ctx)
		if err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	t.Run("ListResources", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.ListResources(ctx, nil)
		if err != nil {
			t.Errorf("ListResources failed: %v", err)
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("ReadResource", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.ReadResource(ctx, "test://resource")
		if err != nil {
			t.Errorf("ReadResource failed: %v", err)
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("Subscribe", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.Subscribe(ctx, "test://resource")
		if err != nil {
			t.Errorf("Subscribe failed: %v", err)
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.Unsubscribe(ctx, "test://resource")
		if err != nil {
			t.Errorf("Unsubscribe failed: %v", err)
		}
	})

	t.Run("ListPrompts", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.ListPrompts(ctx, nil)
		if err != nil {
			t.Errorf("ListPrompts failed: %v", err)
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("GetPrompt", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.GetPrompt(ctx, "test-prompt", nil)
		if err != nil {
			t.Errorf("GetPrompt failed: %v", err)
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("ListTools", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := client.ListTools(ctx, nil)
		if err != nil {
			t.Errorf("ListTools failed: %v", err)
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("CallTool", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		args := map[string]interface{}{
			"param1": "value1",
		}

		result, err := client.CallTool(ctx, "test-tool", args)
		if err != nil {
			t.Errorf("CallTool failed: %v", err)
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("SetLevel", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := client.SetLevel(ctx, LoggingLevelInfo)
		if err != nil {
			t.Errorf("SetLevel failed: %v", err)
		}
	})

	t.Run("Complete", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		ref := PromptReference{
			Type: "ref/prompt",
			Name: "test-prompt",
		}

		arg := CompleteArgument{
			Name:  "test-arg",
			Value: "test-value",
		}

		result, err := client.Complete(ctx, ref, arg)
		if err != nil {
			t.Errorf("Complete failed: %v", err)
		}

		if result == nil {
			t.Error("Expected non-nil result")
		}
	})

	t.Run("Initialization Required", func(t *testing.T) {
		// Create a new uninitialized client
		uninitClient, err := NewStdioMCPClient(mockServerPath)
		if err != nil {
			t.Fatalf("Failed to create uninitialized client: %v", err)
		}
		defer uninitClient.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Try to call a method before initialization
		_, err = uninitClient.ListResources(ctx, nil)
		if err == nil {
			t.Error("Expected error when calling method before initialization")
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.ListResources(ctx, nil)
		if err == nil {
			t.Error("Expected error when context is cancelled")
		}
	})

	t.Run("Invalid Response Handling", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// This assumes the mock server will return an error for an unknown method
		_, err := client.sendRequest(ctx, "invalid_method", nil)
		if err == nil {
			t.Error("Expected error for invalid method")
		}
	})
}

func TestNewStdioMCPClient_Errors(t *testing.T) {
	t.Run("Invalid Command", func(t *testing.T) {
		_, err := NewStdioMCPClient("nonexistent_command")
		if err == nil {
			t.Error("Expected error when creating client with invalid command")
		}
	})
}
