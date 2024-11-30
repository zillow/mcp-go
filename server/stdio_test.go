package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"
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

	origStdin, origStdout, origStderr := os.Stdin, os.Stdout, os.Stderr

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

	os.Stdin, os.Stdout, os.Stderr = stdinR, stdoutW, stderrW

	server := NewDefaultServer("test-server", "1.0.0")
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

	ts.wg.Add(1)
	go func() {
		defer ts.wg.Done()
		if err := ServeStdio(server); err != nil {
			t.Logf("ServeStdio returned error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	return ts
}

func (ts *testStdioServer) cleanup(t *testing.T) {
	t.Helper()

	ts.cancel()
	ts.stdinW.Close()

	done := make(chan struct{})
	go func() {
		ts.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("Server shut down successfully")
	case <-time.After(2 * time.Second):
		t.Error("Server failed to shut down in time")
	}

	ts.stdin.Close()
	ts.stdout.Close()
	ts.stderr.Close()
	ts.stdoutR.Close()
	ts.stderrR.Close()

	os.Stdin, os.Stdout, os.Stderr = ts.origStdin, ts.origStdout, ts.origStderr
}

func (ts *testStdioServer) sendRawRequest(raw string) (*JSONRPCResponse,
	error) {
	responseChan := make(chan *JSONRPCResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		if _, err := ts.stdinW.Write([]byte(raw + "\n")); err != nil {
			errChan <- fmt.Errorf("failed to write request: %w", err)
			return
		}

		scanner := bufio.NewScanner(ts.stdoutR)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				errChan <- fmt.Errorf("failed to read response: %w", err)
			} else {
				errChan <- io.EOF
			}
			return
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
			errChan <- fmt.Errorf("failed to unmarshal response: %w", err)
			return
		}

		responseChan <- &resp
	}()

	select {
	case resp := <-responseChan:
		return resp, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("request timed out")
	}
}

func (ts *testStdioServer) sendRequest(req *JSONRPCRequest) (*JSONRPCResponse,
	error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	return ts.sendRawRequest(string(reqBytes))
}

func TestStdioServer(t *testing.T) {
	ts := setupTestStdioServer(t)
	defer ts.cleanup(t)

	tests := []struct {
		name       string
		rawRequest string
		request    *JSONRPCRequest
		check      func(*testing.T, *JSONRPCResponse)
	}{
		// ... (keep your existing test cases)
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

func TestStdioServerGracefulShutdown(t *testing.T) {
	ts := setupTestStdioServer(t)
	defer ts.cleanup(t)

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

	ts.cancel()
	ts.stdinW.Close()

	done := make(chan struct{})
	go func() {
		ts.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("Server shut down successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Server failed to shut down gracefully")
	}
}
