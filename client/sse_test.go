package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSSEServer implements a test SSE server with message posting endpoint
type mockSSEServer struct {
	server    *httptest.Server
	messages  chan string
	postPath  string
	closeOnce sync.Once
	closed    chan struct{}
}

func newMockSSEServer() *mockSSEServer {
	ms := &mockSSEServer{
		messages: make(chan string, 10),
		postPath: "/post/" + randomString(8),
		closed:   make(chan struct{}),
	}

	mux := http.NewServeMux()

	// SSE endpoint
	mux.HandleFunc("/events", ms.handleSSE)

	// POST endpoint for receiving messages
	mux.HandleFunc(ms.postPath, ms.handlePost)

	ms.server = httptest.NewServer(mux)
	return ms
}

func (ms *mockSSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") != "text/event-stream" {
		http.Error(
			w,
			"Accept header must be text/event-stream",
			http.StatusBadRequest,
		)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Send endpoint event
	fmt.Fprintf(
		w,
		"event: endpoint\ndata: %s%s\n\n",
		ms.server.URL,
		ms.postPath,
	)
	w.(http.Flusher).Flush()

	// Start message streaming
	for {
		select {
		case <-ms.closed:
			return
		case msg := <-ms.messages:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (ms *mockSSEServer) handlePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(
			w,
			"Content-Type must be application/json",
			http.StatusBadRequest,
		)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (ms *mockSSEServer) close() {
	ms.closeOnce.Do(func() {
		close(ms.closed)
		ms.server.Close()
	})
}

func (ms *mockSSEServer) sendMessage(msg *JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case ms.messages <- string(data):
		return nil
	case <-ms.closed:
		return errors.New("server closed")
	}
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestSSETransport_Connect(t *testing.T) {
	tests := []struct {
		name        string
		serverSetup func(*mockSSEServer)
		wantErr     bool
		errorMsg    string
	}{
		{
			name:        "successful connection",
			serverSetup: func(ms *mockSSEServer) {},
			wantErr:     false,
		},
		{
			name: "server closes immediately",
			serverSetup: func(ms *mockSSEServer) {
				ms.close()
			},
			wantErr:  true,
			errorMsg: "failed to connect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockSSEServer()
			defer server.close()

			if tt.serverSetup != nil {
				tt.serverSetup(server)
			}

			transport := NewSSETransport(server.server.URL + "/events")
			err := transport.Connect(context.Background())

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSETransport_ConnectWithContext(t *testing.T) {
	server := newMockSSEServer()
	defer server.close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	transport := NewSSETransport(server.server.URL + "/events")
	err := transport.Connect(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSSETransport_Send(t *testing.T) {
	tests := []struct {
		name    string
		message *JSONRPCMessage
		wantErr bool
	}{
		{
			name: "valid message",
			message: &JSONRPCMessage{
				JSONRPC: "2.0",
				Method:  "test.method",
				Params:  map[string]interface{}{"key": "value"},
				ID:      1,
			},
			wantErr: false,
		},
		{
			name:    "nil message",
			message: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockSSEServer()
			defer server.close()

			transport := NewSSETransport(server.server.URL + "/events")
			err := transport.Connect(context.Background())
			require.NoError(t, err)

			err = transport.Send(context.Background(), tt.message)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSETransport_Receive(t *testing.T) {
	tests := []struct {
		name        string
		sendMessage *JSONRPCMessage
		wantErr     bool
	}{
		{
			name: "valid message",
			sendMessage: &JSONRPCMessage{
				JSONRPC: "2.0",
				Method:  "test.method",
				Params:  map[string]interface{}{"key": "value"},
				ID:      1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newMockSSEServer()
			defer server.close()

			transport := NewSSETransport(server.server.URL + "/events")
			err := transport.Connect(context.Background())
			require.NoError(t, err)

			// Send message through server
			err = server.sendMessage(tt.sendMessage)
			require.NoError(t, err)

			// Receive message
			msg, err := transport.Receive(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, msg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.sendMessage.JSONRPC, msg.JSONRPC)
				assert.Equal(t, tt.sendMessage.Method, msg.Method)
				assert.Equal(t, tt.sendMessage.ID, msg.ID)
			}
		})
	}
}

func TestSSETransport_ReceiveWithContext(t *testing.T) {
	server := newMockSSEServer()
	defer server.close()

	transport := NewSSETransport(server.server.URL + "/events")
	err := transport.Connect(context.Background())
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	msg, err := transport.Receive(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, msg)
}

func TestSSETransport_Disconnect(t *testing.T) {
	server := newMockSSEServer()
	defer server.close()

	transport := NewSSETransport(server.server.URL + "/events")

	// Test disconnecting before connect
	err := transport.Disconnect()
	assert.NoError(t, err)

	// Connect and then disconnect
	err = transport.Connect(context.Background())
	require.NoError(t, err)

	err = transport.Disconnect()
	assert.NoError(t, err)

	// Verify can't receive after disconnect
	_, err = transport.Receive(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestSSETransport_ConcurrentOperations(t *testing.T) {
	server := newMockSSEServer()
	defer server.close()

	transport := NewSSETransport(server.server.URL + "/events")
	err := transport.Connect(context.Background())
	require.NoError(t, err)

	// Start multiple goroutines sending messages
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Add(-1)
			msg := &JSONRPCMessage{
				JSONRPC: "2.0",
				Method:  fmt.Sprintf("test.method.%d", id),
				ID:      id,
			}
			err := transport.Send(context.Background(), msg)
			assert.NoError(t, err)
		}(i)
	}

	// Start multiple goroutines receiving messages
	receivedMsgs := make(chan *JSONRPCMessage, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Add(-1)
			msg, err := transport.Receive(context.Background())
			if err == nil {
				receivedMsgs <- msg
			}
		}()

		// Send test messages through server
		msg := &JSONRPCMessage{
			JSONRPC: "2.0",
			Method:  fmt.Sprintf("test.method.%d", i),
			ID:      i,
		}
		server.sendMessage(msg)
	}

	// Wait for all operations to complete
	wg.Wait()
	close(receivedMsgs)

	// Verify received messages
	received := make(map[int]bool)
	for msg := range receivedMsgs {
		id, ok := msg.ID.(int)
		assert.True(t, ok)
		received[id] = true
	}
	assert.Len(t, received, 5)
}

func TestSSETransport_CustomHTTPClient(t *testing.T) {
	server := newMockSSEServer()
	defer server.close()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	transport := NewSSETransport(
		server.server.URL+"/events",
		WithHTTPClient(client),
	)

	err := transport.Connect(context.Background())
	assert.NoError(t, err)
}

func TestSSEScanner(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []SSEEvent
		wantErr  bool
	}{
		{
			name: "valid events",
			input: "event: endpoint\ndata: http://example.com/post\n\n" +
				"event: message\ndata: {\"jsonrpc\":\"2.0\"}\n\n",
			expected: []SSEEvent{
				{Type: "endpoint", Data: "http://example.com/post"},
				{Type: "message", Data: "{\"jsonrpc\":\"2.0\"}"},
			},
			wantErr: false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
			wantErr:  false,
		},
		{
			name:  "malformed event",
			input: "event: test\ndata",
			expected: []SSEEvent{
				{Type: "test", Data: ""},
			},
			wantErr: false,
		},
		{
			name:  "multi-line data",
			input: "event: test\ndata: line1\ndata: line2\n\n",
			expected: []SSEEvent{
				{Type: "test", Data: "line1\nline2"},
			},
			wantErr: false,
		},
		{
			name:  "comment lines",
			input: ": this is a comment\nevent: test\ndata: value\n\n",
			expected: []SSEEvent{
				{Type: "test", Data: "value"},
			},
			wantErr: false,
		},
		{
			name:  "no trailing newlines",
			input: "event: test\ndata: value",
			expected: []SSEEvent{
				{Type: "test", Data: "value"},
			},
			wantErr: false,
		},
		{
			name:  "malformed lines with colons",
			input: "event : test\ndata : value\n\n",
			expected: []SSEEvent{
				{Type: "test", Data: "value"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := NewEventScanner(strings.NewReader(tt.input))
			var events []SSEEvent

			for scanner.Scan() {
				events = append(events, scanner.Event())
			}

			if tt.wantErr {
				assert.Error(t, scanner.Err())
			} else {
				assert.NoError(t, scanner.Err())
				assert.Equal(t, tt.expected, events)
			}
		})
	}
}
