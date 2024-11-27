package client

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTransport implements the Transport interface for testing
type MockTransport struct {
	mock.Mock
}

func (m *MockTransport) Connect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTransport) Disconnect() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockTransport) Send(ctx context.Context, msg *JSONRPCMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func (m *MockTransport) Receive(ctx context.Context) (*JSONRPCMessage, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*JSONRPCMessage), args.Error(1)
}

// Test setup helper
func setupTestClient() (*Client, *MockTransport) {
	transport := new(MockTransport)
	config := ClientConfig{
		Implementation: shared.Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
		Capabilities: shared.ClientCapabilities{
			Sampling: make(map[string]interface{}),
		},
	}
	client := New(config, transport)
	return client, transport
}

func TestNew(t *testing.T) {
	client, _ := setupTestClient()

	assert.NotNil(t, client)
	assert.Equal(t, "test-client", client.config.Implementation.Name)
	assert.Equal(t, "1.0.0", client.config.Implementation.Version)
	assert.NotNil(t, client.handlers)
}

func TestConnect(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockTransport)
		expectedError string
	}{
		{
			name: "successful connection",
			setupMock: func(mt *MockTransport) {
				mt.On("Connect", mock.Anything).Return(nil)
				mt.On("Send", mock.Anything, mock.MatchedBy(func(msg *JSONRPCMessage) bool {
					return msg.Method == "initialize"
				})).
					Return(nil)
				mt.On("Receive", mock.Anything).Return(&JSONRPCMessage{
					JSONRPC: "2.0",
					ID:      1,
					Result: map[string]interface{}{
						"serverInfo": map[string]interface{}{
							"name":    "test-server",
							"version": "1.0.0",
						},
					},
				}, nil)
				mt.On("Send", mock.Anything, mock.MatchedBy(func(msg *JSONRPCMessage) bool {
					return msg.Method == "notifications/initialized"
				})).
					Return(nil)
			},
			expectedError: "",
		},
		{
			name: "connection error",
			setupMock: func(mt *MockTransport) {
				mt.On("Connect", mock.Anything).
					Return(errors.New("connection failed"))
			},
			expectedError: "connection failed",
		},
		{
			name: "initialize send error",
			setupMock: func(mt *MockTransport) {
				mt.On("Connect", mock.Anything).Return(nil)
				mt.On("Send", mock.Anything, mock.Anything).
					Return(errors.New("send failed"))
			},
			expectedError: "send failed",
		},
		{
			name: "initialize receive error",
			setupMock: func(mt *MockTransport) {
				mt.On("Connect", mock.Anything).Return(nil)
				mt.On("Send", mock.Anything, mock.Anything).Return(nil)
				mt.On("Receive", mock.Anything).
					Return(nil, errors.New("receive failed"))
			},
			expectedError: "receive failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, transport := setupTestClient()
			tt.setupMock(transport)

			err := client.Connect(context.Background())

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			transport.AssertExpectations(t)
		})
	}
}

func TestRegisterNotificationHandler(t *testing.T) {
	client, _ := setupTestClient()

	// Test registering a handler
	testMethod := "test/notification"
	testHandler := func(ctx context.Context, msg *JSONRPCMessage) error {
		return nil
	}

	client.RegisterNotificationHandler(testMethod, testHandler)

	// Verify handler was registered
	client.handlersmu.RLock()
	handler, exists := client.handlers[testMethod]
	client.handlersmu.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, handler)
}

func TestHandleNotification(t *testing.T) {
	client, _ := setupTestClient()

	var handlerCalled bool
	var handlerMsg *JSONRPCMessage
	var handlerMu sync.Mutex

	// Register test handler
	testMethod := "test/notification"
	testHandler := func(ctx context.Context, msg *JSONRPCMessage) error {
		handlerMu.Lock()
		handlerCalled = true
		handlerMsg = msg
		handlerMu.Unlock()
		return nil
	}

	client.RegisterNotificationHandler(testMethod, testHandler)

	// Test notification handling
	testNotification := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  testMethod,
		Params: map[string]interface{}{
			"test": "data",
		},
	}

	err := client.handleNotification(context.Background(), testNotification)
	assert.NoError(t, err)

	// Allow some time for async handler execution
	time.Sleep(100 * time.Millisecond)

	handlerMu.Lock()
	assert.True(t, handlerCalled)
	assert.Equal(t, testNotification, handlerMsg)
	handlerMu.Unlock()
}

func TestRequestIDGeneration(t *testing.T) {
	client, _ := setupTestClient()

	// Test sequential ID generation
	id1 := client.nextRequestID()
	id2 := client.nextRequestID()
	id3 := client.nextRequestID()

	// Convert to int64 for comparison
	id1Int, ok := id1.(int64)
	assert.True(t, ok)
	id2Int, ok := id2.(int64)
	assert.True(t, ok)
	id3Int, ok := id3.(int64)
	assert.True(t, ok)

	// Verify sequential increment
	assert.Equal(t, int64(1), id1Int)
	assert.Equal(t, int64(2), id2Int)
	assert.Equal(t, int64(3), id3Int)
}

func TestConcurrentRequestIDGeneration(t *testing.T) {
	client, _ := setupTestClient()

	numGoroutines := 100
	idsPerGoroutine := 100

	var wg sync.WaitGroup
	idSet := sync.Map{}

	// Generate IDs concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id := client.nextRequestID()
				// Verify uniqueness
				_, loaded := idSet.LoadOrStore(id, true)
				assert.False(t, loaded, "Duplicate ID generated: %v", id)
			}
		}()
	}

	wg.Wait()

	// Count total unique IDs
	count := 0
	idSet.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	assert.Equal(t, numGoroutines*idsPerGoroutine, count)
}

func TestInvalidNotificationHandler(t *testing.T) {
	client, _ := setupTestClient()

	// Test handling notification with no registered handler
	testNotification := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "unregistered/method",
		Params:  map[string]interface{}{},
	}

	err := client.handleNotification(context.Background(), testNotification)
	assert.NoError(t, err) // Should not error for unhandled notifications
}
