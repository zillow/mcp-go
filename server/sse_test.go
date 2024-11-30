package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSSEConnection(t *testing.T) {
	mcpServer := NewDefaultServer("test", "1.0.0")
	_, testServer := NewTestServer(mcpServer)
	defer testServer.Close()

	// Connect to SSE endpoint
	resp, err := http.Get(testServer.URL + "/sse")
	assert.NoError(t, err)
	defer resp.Body.Close()

	// Check headers
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))

	// Read endpoint event
	reader := bufio.NewReader(resp.Body)
	eventLine, err := reader.ReadString('\n')
	assert.NoError(t, err)
	assert.Equal(t, "event: endpoint\n", eventLine)

	dataLine, err := reader.ReadString('\n')
	assert.NoError(t, err)
	assert.Contains(t, dataLine, "/message?sessionId=")
}

func TestSessionSpecificMessages(t *testing.T) {
	mcpServer := NewDefaultServer("test", "1.0.0")
	_, testServer := NewTestServer(mcpServer)
	defer testServer.Close()

	// Connect two clients
	resp1, err := http.Get(testServer.URL + "/sse")
	assert.NoError(t, err)
	defer resp1.Body.Close()

	resp2, err := http.Get(testServer.URL + "/sse")
	assert.NoError(t, err)
	defer resp2.Body.Close()

	// Get session IDs
	reader1 := bufio.NewReader(resp1.Body)
	reader2 := bufio.NewReader(resp2.Body)

	_, _ = reader1.ReadString('\n')
	dataLine1, _ := reader1.ReadString('\n')
	sessionID1 := strings.Split(strings.Split(dataLine1, "sessionId=")[1],
		"\n")[0]

	_, _ = reader2.ReadString('\n')
	dataLine2, _ := reader2.ReadString('\n')
	sessionID2 := strings.Split(strings.Split(dataLine2, "sessionId=")[1],
		"\n")[0]

	// Create message channels
	messages1 := make(chan string)
	messages2 := make(chan string)

	go readSSEMessages(reader1, messages1)
	go readSSEMessages(reader2, messages2)

	// Send initialize requests for both clients
	request1 := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "abc123",
		Method:  "initialize",
		Params: json.RawMessage(`{
                "clientInfo": {
                    "name": "test-client-1",
                    "version": "1.0.0"
                },
                "capabilities": {},
                "protocolVersion": "2024-11-05"
            }`),
	}

	request2 := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "def456",
		Method:  "initialize",
		Params: json.RawMessage(`{
                "clientInfo": {
                    "name": "test-client-2",
                    "version": "1.0.0"
                },
                "capabilities": {},
                "protocolVersion": "2024-11-05"
            }`),
	}

	// Send requests
	sendJSONRPCRequest(t, testServer.URL, sessionID1, request1)
	sendJSONRPCRequest(t, testServer.URL, sessionID2, request2)

	// Verify responses go to correct clients
	verifyJSONRPCResponse(t, messages1, "abc123")
	verifyJSONRPCResponse(t, messages2, "def456")
}

func TestInvalidJSONRPC(t *testing.T) {
	mcpServer := NewDefaultServer("test", "1.0.0")
	_, testServer := NewTestServer(mcpServer)
	defer testServer.Close()

	// Get valid session ID
	resp, err := http.Get(testServer.URL + "/sse")
	assert.NoError(t, err)
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	_, _ = reader.ReadString('\n')
	dataLine, _ := reader.ReadString('\n')
	sessionID := strings.Split(strings.Split(dataLine, "sessionId=")[1], "\n")[0]

	// Test cases
	tests := []struct {
		name           string
		request        interface{}
		expectedError  int
		expectedStatus int
	}{
		{
			name:           "Invalid JSON",
			request:        "invalid json",
			expectedError:  -32700,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Missing JSONRPC version",
			request: map[string]interface{}{
				"id":     1,
				"method": "test",
			},
			expectedError:  -32600,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Wrong JSONRPC version",
			request: map[string]interface{}{
				"jsonrpc": "1.0",
				"id":      1,
				"method":  "test",
			},
			expectedError:  -32600,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			if str, ok := tc.request.(string); ok {
				body = []byte(str)
			} else {
				body, _ = json.Marshal(tc.request)
			}

			resp, err := http.Post(
				fmt.Sprintf(
					"%s/message?sessionId=%s",
					testServer.URL,
					sessionID,
				),
				"application/json",
				bytes.NewBuffer(body),
			)
			assert.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			var response JSONRPCResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			assert.NoError(t, err)
			assert.NotNil(t, response.Error)
			assert.Equal(t, tc.expectedError, response.Error.Code)
		})
	}
}

func TestInvalidSession(t *testing.T) {
	mcpServer := NewDefaultServer("test", "1.0.0")
	_, testServer := NewTestServer(mcpServer)
	defer testServer.Close()

	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "ping",
		Params:  json.RawMessage("{}"),
	}
	jsonBody, _ := json.Marshal(request)

	// Test missing session ID
	resp, err := http.Post(
		testServer.URL+"/message",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var response JSONRPCResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32602, response.Error.Code)

	// Test invalid session ID
	resp, err = http.Post(
		testServer.URL+"/message?sessionId=invalid",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32602, response.Error.Code)
}

func TestMethodNotAllowed(t *testing.T) {
	mcpServer := NewDefaultServer("test", "1.0.0")
	_, testServer := NewTestServer(mcpServer)
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/message")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var response JSONRPCResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.NotNil(t, response.Error)
	assert.Equal(t, -32600, response.Error.Code)
}

// Helper functions
func readSSEMessages(reader *bufio.Reader, messageChan chan<- string) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			close(messageChan)
			return
		}
		if strings.HasPrefix(line, "data: ") {
			messageChan <- strings.TrimPrefix(line, "data: ")
		}
	}
}

func sendJSONRPCRequest(
	t *testing.T,
	serverURL, sessionID string,
	request JSONRPCRequest,
) {
	jsonBody, err := json.Marshal(request)
	assert.NoError(t, err)

	resp, err := http.Post(
		fmt.Sprintf("%s/message?sessionId=%s", serverURL, sessionID),
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var response JSONRPCResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, request.ID, response.ID)
	assert.Nil(t, response.Error)
}

func verifyJSONRPCResponse(
	t *testing.T,
	messageChan chan string,
	expectedID interface{},
) {
	select {
	case message := <-messageChan:
		var response JSONRPCResponse
		err := json.Unmarshal([]byte(message), &response)
		assert.NoError(t, err)
		assert.Equal(t, "2.0", response.JSONRPC)
		assert.Equal(t, expectedID, response.ID)
		assert.Nil(t, response.Error)

		// Verify it's an initialize response
		result := response.Result.(map[string]interface{})
		serverInfo := result["serverInfo"].(map[string]interface{})
		assert.Equal(t, "test", serverInfo["name"])
	case <-time.After(time.Second * 5):
		t.Fatal("Timeout waiting for SSE message")
	}
}
