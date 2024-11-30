package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

type testStdioServer struct {
	server     MCPServer
	stdin      *os.File
	stdout     *os.File
	stderr     *os.File
	stdinW     *os.File
	stdoutR    *os.File
	stderrR    *os.File
	origStdin  *os.File
	origStdout *os.File
	origStderr *os.File
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func setupTestStdioServer(t *testing.T) *testStdioServer {
	t.Helper()

	// Save original stdio
	origStdin := os.Stdin
	origStdout := os.Stdout
	origStderr := os.Stderr

	// Create pipes for stdin, stdout, stderr
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}

	// Replace stdio with pipes
	os.Stdin = stdinR
	os.Stdout = stdoutW
	os.Stderr = stderrW

	server := NewDefaultServer("test-server", "1.0.0")

	// Create a context with cancel for the server
	_, cancel := context.WithCancel(context.Background())

	ts := &testStdioServer{
		server:     server,
		stdin:      stdinR,
		stdout:     stdoutW,
		stderr:     stderrW,
		stdinW:     stdinW,
		stdoutR:    stdoutR,
		stderrR:    stderrR,
		origStdin:  origStdin,
		origStdout: origStdout,
		origStderr: origStderr,
		cancel:     cancel,
	}

	// Start the server in a goroutine
	ts.wg.Add(1)
	go func() {
		defer ts.wg.Done()
		ServeStdio(server)
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	return ts
}

func (ts *testStdioServer) cleanup() {
	// Cancel context and close stdin to signal server shutdown
	ts.cancel()
	ts.stdinW.Close()

	// Wait for server to finish with timeout
	done := make(chan struct{})
	go func() {
		ts.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Server shut down successfully
	case <-time.After(2 * time.Second):
		// Server failed to shut down in time
	}

	// Close all pipes
	ts.stdin.Close()
	ts.stdout.Close()
	ts.stderr.Close()
	ts.stdoutR.Close()
	ts.stderrR.Close()

	// Restore original stdio
	os.Stdin = ts.origStdin
	os.Stdout = ts.origStdout
	os.Stderr = ts.origStderr
}

func TestStdioServer(t *testing.T) {
	ts := setupTestStdioServer(t)
	defer ts.cleanup()

	tests := []struct {
		name       string
		rawRequest string // Use raw string for invalid JSON cases
		request    *JSONRPCRequest
		check      func(*testing.T, *JSONRPCResponse)
	}{
		{
			name: "Initialize",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params: json.RawMessage(`{
                        "capabilities": {},
                        "clientInfo": {"name": "test-client", "version": "1.0"},
                        "protocolVersion": "1.0"
                    }`),
			},
			check: func(t *testing.T, resp *JSONRPCResponse) {
				if resp.Error != nil {
					t.Fatalf("Unexpected error: %v", resp.Error)
				}
				var result mcp.InitializeResult
				b, err := json.Marshal(resp.Result)
				if err != nil {
					t.Fatalf("Failed to marshal result: %v", err)
				}
				if err := json.Unmarshal(b, &result); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}
				if result.ServerInfo.Name != "test-server" {
					t.Errorf(
						"Expected server name 'test-server', got '%s'",
						result.
							ServerInfo.Name,
					)
				}
			},
		},
		{
			name: "Invalid JSON-RPC Version",
			request: &JSONRPCRequest{
				JSONRPC: "1.0",
				ID:      2,
				Method:  "ping",
			},
			check: func(t *testing.T, resp *JSONRPCResponse) {
				if resp.Error == nil {
					t.Error("Expected error response")
					return
				}
				if resp.Error.Code != -32600 {
					t.Errorf(
						"Expected error code -32600, got %d",
						resp.Error.Code,
					)
				}
			},
		},
		{
			name:       "Invalid JSON",
			rawRequest: `{invalid json}`,
			check: func(t *testing.T, resp *JSONRPCResponse) {
				if resp.Error == nil {
					t.Error("Expected error response")
					return
				}
				if resp.Error.Code != -32700 {
					t.Errorf(
						"Expected error code -32700, got %d",
						resp.Error.Code,
					)
				}
			},
		},
		{
			name: "Method Not Found",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      4,
				Method:  "invalid_method",
			},
			check: func(t *testing.T, resp *JSONRPCResponse) {
				if resp.Error == nil {
					t.Error("Expected error response")
					return
				}
				if !strings.Contains(resp.Error.Message, "method not found") {
					t.Errorf(
						"Expected 'method not found' error, got '%s'",
						resp.Error.
							Message,
					)
				}
			},
		},
		{
			name: "Ping Success",
			request: &JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      5,
				Method:  "ping",
			},
			check: func(t *testing.T, resp *JSONRPCResponse) {
				if resp.Error != nil {
					t.Errorf("Unexpected error: %v", resp.Error)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp *JSONRPCResponse
			var err error

			if tt.rawRequest != "" {
				resp, err = ts.sendRawRequest(tt.rawRequest)
			} else {
				resp, err = ts.sendRequest(tt.request)
			}

			if err != nil {
				t.Fatalf("sendRequest() failed: %v", err)
			}
			if resp == nil {
				t.Fatal("Expected response, got nil")
			}
			tt.check(t, resp)
		})
	}
}

// Add a new method to send raw requests
func (ts *testStdioServer) sendRawRequest(
	raw string,
) (*JSONRPCResponse, error) {
	// Create a channel for the response
	responseChan := make(chan *JSONRPCResponse, 1)
	errChan := make(chan error, 1)

	// Send request in a goroutine
	go func() {
		// Send raw request
		if _, err := ts.stdinW.Write([]byte(raw + "\n")); err != nil {
			errChan <- err
			return
		}

		// Read response with timeout
		scanner := bufio.NewScanner(ts.stdoutR)
		if !scanner.Scan() {
			errChan <- scanner.Err()
			return
		}

		// Parse response
		var resp JSONRPCResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			errChan <- err
			return
		}

		responseChan <- &resp
	}()

	// Wait for response with timeout
	select {
	case resp := <-responseChan:
		return resp, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("request timed out")
	}
}

func (ts *testStdioServer) sendRequest(
	req *JSONRPCRequest,
) (*JSONRPCResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return ts.sendRawRequest(string(reqBytes))
}

func TestStdioServerGracefulShutdown(t *testing.T) {
	ts := setupTestStdioServer(t)

	// Send a request
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ping",
	}
	resp, err := ts.sendRequest(&req)
	if err != nil {
		t.Fatalf("Initial request failed: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("Initial request returned error: %v", resp.Error)
	}

	// Close stdin and ensure server shuts down gracefully
	ts.stdinW.Close()

	// Wait for shutdown with timeout
	done := make(chan struct{})
	go func() {
		ts.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Server shut down successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Server failed to shut down gracefully")
	}
}

func TestStdioServerStderr(t *testing.T) {
	ts := setupTestStdioServer(t)
	defer ts.cleanup()

	// Send an invalid request to trigger error logging
	req := JSONRPCRequest{
		JSONRPC: "1.0", // Invalid version
		ID:      1,
		Method:  "ping",
	}
	_, err := ts.sendRequest(&req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Read stderr
	errBuf := make([]byte, 1024)
	n, err := ts.stderrR.Read(errBuf)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read stderr: %v", err)
	}

	// Check that something was logged to stderr
	if n == 0 {
		t.Error("Expected error log in stderr, got nothing")
	}
}
