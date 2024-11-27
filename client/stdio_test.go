package client

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer is a simple echo server for testing
const mockServerScript = `#!/bin/bash
    while IFS= read -r line; do
        echo "$line"
        echo "log message" >&2
    done
    `

func setupMockServer(t *testing.T) string {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "stdio-test-*")
	require.NoError(t, err)

	// Create mock server script
	scriptPath := filepath.Join(tmpDir, "mock-server.sh")
	err = os.WriteFile(scriptPath, []byte(mockServerScript), 0755)
	require.NoError(t, err)

	return scriptPath
}

func TestStdioTransport_Connect(t *testing.T) {
	scriptPath := setupMockServer(t)
	defer os.RemoveAll(filepath.Dir(scriptPath))

	transport := NewStdioTransport(scriptPath, nil)
	err := transport.Connect(context.Background())
	assert.NoError(t, err)
	assert.True(t, transport.IsConnected())

	err = transport.Disconnect()
	assert.NoError(t, err)
}

func TestStdioTransport_SendReceive(t *testing.T) {
	scriptPath := setupMockServer(t)
	defer os.RemoveAll(filepath.Dir(scriptPath))

	transport := NewStdioTransport(scriptPath, nil)
	err := transport.Connect(context.Background())
	require.NoError(t, err)
	defer transport.Disconnect()

	// Send test message
	testMsg := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test.method",
		Params:  map[string]interface{}{"key": "value"},
		ID:      1,
	}

	err = transport.Send(context.Background(), testMsg)
	assert.NoError(t, err)

	// Receive response
	msg, err := transport.Receive(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, testMsg.JSONRPC, msg.JSONRPC)
	assert.Equal(t, testMsg.Method, msg.Method)
	assert.Equal(t, testMsg.ID, msg.ID)
}

func TestStdioTransport_ContextCancellation(t *testing.T) {
	scriptPath := setupMockServer(t)
	defer os.RemoveAll(filepath.Dir(scriptPath))

	transport := NewStdioTransport(scriptPath, nil)
	err := transport.Connect(context.Background())
	require.NoError(t, err)
	defer transport.Disconnect()

	// Test context cancellation during receive
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = transport.Receive(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestStdioTransport_StderrHandler(t *testing.T) {
	scriptPath := setupMockServer(t)
	defer os.RemoveAll(filepath.Dir(scriptPath))

	var mu sync.Mutex
	var stderrOutput []string
	transport := NewStdioTransport(
		scriptPath,
		nil,
		WithStdioStderrHandler(func(line string) {
			mu.Lock()
			stderrOutput = append(stderrOutput, line)
			mu.Unlock()
		}),
	)

	err := transport.Connect(context.Background())
	require.NoError(t, err)
	defer transport.Disconnect()

	// Send a message to trigger stderr output
	testMsg := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "test",
	}
	err = transport.Send(context.Background(), testMsg)
	assert.NoError(t, err)

	// Wait for stderr handler to receive output
	success := assert.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(stderrOutput) > 0
	}, 2*time.Second, 100*time.Millisecond, "stderr output not received")

	if success {
		mu.Lock()
		assert.Contains(t, stderrOutput, "log message")
		mu.Unlock()
	}
}

func TestStdioTransport_ProcessExit(t *testing.T) {
	scriptPath := setupMockServer(t)
	defer os.RemoveAll(filepath.Dir(scriptPath))

	transport := NewStdioTransport(scriptPath, nil)
	err := transport.Connect(context.Background())
	require.NoError(t, err)

	// Close stdin to make the process exit
	transport.Disconnect()

	// Trying to send after process exit should fail
	err = transport.Send(context.Background(), &JSONRPCMessage{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}
