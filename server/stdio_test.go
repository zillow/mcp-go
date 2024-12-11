package server

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"testing"
)

func TestStdioServer(t *testing.T) {
	t.Run("Can instantiate", func(t *testing.T) {
		mcpServer := NewMCPServer("test", "1.0.0")
		stdioServer := NewStdioServer(mcpServer)

		if stdioServer.server == nil {
			t.Error("MCPServer should not be nil")
		}
		if stdioServer.errLogger == nil {
			t.Error("errLogger should not be nil")
		}
	})

	t.Run("Can send and receive messages", func(t *testing.T) {
		// Create pipes for stdin and stdout
		stdinReader, stdinWriter := io.Pipe()
		stdoutReader, stdoutWriter := io.Pipe()

		// Create server
		mcpServer := NewMCPServer("test", "1.0.0",
			WithResourceCapabilities(true, true),
		)
		stdioServer := NewStdioServer(mcpServer)
		stdioServer.SetErrorLogger(log.New(io.Discard, "", 0))

		// Create context with cancel
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start server in goroutine
		go func() {
			if err := stdioServer.Listen(ctx, stdinReader, stdoutWriter); err != nil && err != io.EOF {
				t.Errorf("server error: %v", err)
			}
		}()

		// Create test message
		initRequest := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		// Send request
		go func() {
			requestBytes, err := json.Marshal(initRequest)
			if err != nil {
				t.Error(err)
				return
			}
			_, err = stdinWriter.Write(append(requestBytes, '\n'))
			if err != nil {
				t.Error(err)
				return
			}
			stdinWriter.Close() // Close the writer after sending the message
		}()

		// Read response
		scanner := bufio.NewScanner(stdoutReader)
		if !scanner.Scan() {
			t.Fatal("failed to read response")
		}
		responseBytes := scanner.Bytes()

		var response map[string]interface{}
		if err := json.Unmarshal(responseBytes, &response); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Verify response structure
		if response["jsonrpc"] != "2.0" {
			t.Errorf(
				"expected jsonrpc version 2.0, got %v",
				response["jsonrpc"],
			)
		}
		if response["id"].(float64) != 1 {
			t.Errorf("expected id 1, got %v", response["id"])
		}
		if response["error"] != nil {
			t.Errorf("unexpected error in response: %v", response["error"])
		}
		if response["result"] == nil {
			t.Error("expected result in response")
		}

		// Clean up
		cancel()
		stdoutWriter.Close()
	})
}
