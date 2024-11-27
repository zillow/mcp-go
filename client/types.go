package client

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/shared"
)

// RequestID represents a uniquely identifying ID for a request in JSON-RPC
type RequestID interface{} // can be string or integer

// JSONRPCMessage represents a JSON-RPC 2.0 message
type JSONRPCMessage struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      RequestID     `json:"id,omitempty"`
	Method  string        `json:"method,omitempty"`
	Params  interface{}   `json:"params,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ClientConfig represents configuration options for the MCP client
type ClientConfig struct {
	Implementation shared.Implementation
	Capabilities   shared.ClientCapabilities
}

// TransportConfig represents configuration options for the transport layer
type TransportConfig struct {
	// Add transport-specific configuration options here
}

// UnmarshalJSON implements custom unmarshaling for JSONRPCMessage
func (m *JSONRPCMessage) UnmarshalJSON(data []byte) error {
	type Alias JSONRPCMessage
	aux := &struct {
		ID json.RawMessage `json:"id,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle ID field specially
	if aux.ID != nil {
		// Try as integer first
		var i int
		if err := json.Unmarshal(aux.ID, &i); err == nil {
			m.ID = i
			return nil
		}

		// Try as string next
		var s string
		if err := json.Unmarshal(aux.ID, &s); err == nil {
			m.ID = s
			return nil
		}

		// If neither, return the original error
		return fmt.Errorf("id must be an integer or string")
	}

	return nil
}

// MarshalJSON implements custom marshaling for JSONRPCMessage
func (m JSONRPCMessage) MarshalJSON() ([]byte, error) {
	type Alias JSONRPCMessage
	return json.Marshal((Alias)(m))
}
