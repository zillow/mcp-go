package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/google/uuid"
)

type SSEServer struct {
	mcpServer MCPServer
	baseURL   string
	sessions  sync.Map
	srv       *http.Server
}

type sseSession struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	done    chan struct{}
}

func NewSSEServer(mcpServer MCPServer, baseURL string) *SSEServer {
	return &SSEServer{
		mcpServer: mcpServer,
		baseURL:   baseURL,
	}
}

// NewTestServer creates a test server for testing purposes
// It returns the SSEServer and a test server that can be closed when done
func NewTestServer(mcpServer MCPServer) (*SSEServer, *httptest.Server) {
	// Create SSE server with test server's URL as base
	sseServer := &SSEServer{
		mcpServer: mcpServer,
	}

	// Create test HTTP server
	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/sse":
				sseServer.handleSSE(w, r)
			case "/message":
				sseServer.handleMessage(w, r)
			default:
				http.NotFound(w, r)
			}
		}),
	)

	// Set base URL from test server
	sseServer.baseURL = testServer.URL

	return sseServer, testServer
}

func (s *SSEServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", s.handleSSE)
	mux.HandleFunc("/message", s.handleMessage)

	s.srv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s.srv.ListenAndServe()
}

func (s *SSEServer) Shutdown(ctx context.Context) error {
	if s.srv != nil {
		// Clean up sessions
		s.sessions.Range(func(key, value interface{}) bool {
			if session, ok := value.(*sseSession); ok {
				close(session.done)
			}
			s.sessions.Delete(key)
			return true
		})

		return s.srv.Shutdown(ctx)
	}
	return nil
}

func (s *SSEServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create new session
	sessionID := uuid.New().String()
	log.Info("New client connected", "ID", sessionID)
	session := &sseSession{
		writer:  w,
		flusher: flusher,
		done:    make(chan struct{}),
	}

	// Store session
	s.sessions.Store(sessionID, session)
	defer s.sessions.Delete(sessionID)

	// Send endpoint event
	messageEndpoint := fmt.Sprintf("%s/message?sessionId=%s", s.baseURL,
		sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", messageEndpoint)
	flusher.Flush()

	// Keep connection alive until client disconnects
	<-r.Context().Done()
	close(session.done)
}

func (s *SSEServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSONRPCError(w, nil, -32600, "Method not allowed")
		return
	}

	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		s.writeJSONRPCError(w, nil, -32602, "Missing sessionId")
		return
	}

	sessionI, ok := s.sessions.Load(sessionID)
	if !ok {
		s.writeJSONRPCError(w, nil, -32602, "Invalid session ID")
		return
	}
	session := sessionI.(*sseSession)

	// Parse JSONRPC request
	var request mcp.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeJSONRPCError(w, nil, -32700, "Parse error")
		return
	}

	// Process request through MCPServer
	response := s.mcpServer.Request(r.Context(), request)

	// Send response via SSE
	eventData, _ := json.Marshal(response)
	fmt.Fprintf(session.writer, "event: message\ndata: %s\n\n", eventData)
	session.flusher.Flush()

	// Send HTTP response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

func (s *SSEServer) writeJSONRPCError(w http.ResponseWriter, id interface{},
	code int, message string) {
	response := mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

// SendEventToSession sends an event to a specific session
func (s *SSEServer) SendEventToSession(
	sessionID string,
	event interface{},
) error {
	sessionI, ok := s.sessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	session := sessionI.(*sseSession)

	eventData, err := json.Marshal(event)
	if err != nil {
		return err
	}

	select {
	case <-session.done:
		return fmt.Errorf("session closed")
	default:
		fmt.Fprintf(session.writer, "event: message\ndata: %s\n\n", eventData)
		session.flusher.Flush()
		return nil
	}
}
