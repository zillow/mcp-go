package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// StdioServer wraps a DefaultServer and handles stdio communication
type StdioServer struct {
	server    *DefaultServer
	sigChan   chan os.Signal
	errLogger *log.Logger
}

// ServeStdio creates a stdio server wrapper around an existing DefaultServer
func ServeStdio(server *DefaultServer) error {
	s := &StdioServer{
		server:    server,
		sigChan:   make(chan os.Signal, 1),
		errLogger: log.New(os.Stderr, "", log.LstdFlags),
	}
	return s.serve()
}

// serve starts the stdio server
func (s *StdioServer) serve() error {
	// Set up signal handling
	signal.Notify(s.sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Create buffered reader for stdin
	reader := bufio.NewReader(os.Stdin)

	// Create a context that's cancelled when we receive a signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-s.sigChan
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err := s.handleNextMessage(ctx, reader); err != nil {
				if err == io.EOF {
					return nil
				}
				s.errLogger.Printf("Error handling message: %v", err)
			}
		}
	}
}

func (s *StdioServer) handleNextMessage(
	ctx context.Context,
	reader *bufio.Reader,
) error {
	// Read the next line
	line, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	// Parse the JSON-RPC request
	var request JSONRPCRequest
	if err := json.Unmarshal([]byte(line), &request); err != nil {
		s.writeError(nil, -32700, "Parse error")
		return fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}

	// Validate JSON-RPC version
	if request.JSONRPC != "2.0" {
		s.writeError(request.ID, -32600, "Invalid Request")
		return fmt.Errorf("invalid JSON-RPC version")
	}

	// Handle the request using the wrapped server
	result, err := s.server.Request(ctx, request.Method, request.Params)
	if err != nil {
		s.writeError(request.ID, -32603, err.Error())
		return fmt.Errorf("request handling error: %w", err)
	}

	// Send the response
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
		Result:  result,
	}

	if err := s.writeResponse(response); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
}

func (s *StdioServer) writeError(id interface{}, code int, message string) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{
			Code:    code,
			Message: message,
		},
	}
	s.writeResponse(response)
}

func (s *StdioServer) writeResponse(response JSONRPCResponse) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return err
	}

	// Write response followed by newline
	if _, err := fmt.Fprintf(os.Stdout, "%s\n", responseBytes); err != nil {
		return err
	}

	return nil
}
