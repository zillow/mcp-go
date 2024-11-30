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

// StdioServer wraps a MCPServer and handles stdio communication
type StdioServer struct {
	server    MCPServer
	sigChan   chan os.Signal
	errLogger *log.Logger
	done      chan struct{}
}

// ServeStdio creates a stdio server wrapper around an existing MCPServer
func ServeStdio(server MCPServer) error {
	s := &StdioServer{
		server:    server,
		sigChan:   make(chan os.Signal, 1),
		errLogger: log.New(os.Stderr, "", log.LstdFlags),
		done:      make(chan struct{}),
	}

	// Set up signal handling
	signal.Notify(s.sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Handle shutdown in a separate goroutine
	go func() {
		<-s.sigChan
		close(s.done)
	}()

	return s.serve()
}

func (s *StdioServer) serve() error {
	reader := bufio.NewReader(os.Stdin)

	// Create a context that's cancelled when we receive a signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown in a separate goroutine
	go func() {
		<-s.done
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// Use a goroutine to make the read cancellable
			readChan := make(chan string, 1)
			errChan := make(chan error, 1)

			go func() {
				line, err := reader.ReadString('\n')
				if err != nil {
					errChan <- err
					return
				}
				readChan <- line
			}()

			select {
			case <-ctx.Done():
				return nil
			case err := <-errChan:
				if err == io.EOF {
					return nil
				}
				s.errLogger.Printf("Error reading input: %v", err)
				return err
			case line := <-readChan:
				if err := s.handleMessage(ctx, line); err != nil {
					if err == io.EOF {
						return nil
					}
					s.errLogger.Printf("Error handling message: %v", err)
				}
			}
		}
	}
}

func (s *StdioServer) handleMessage(ctx context.Context, line string) error {
	// Parse the JSON-RPC request
	var request JSONRPCRequest
	if err := json.Unmarshal([]byte(line), &request); err != nil {
		s.writeResponse(JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}{
				Code:    -32700,
				Message: "Parse error",
			},
		})
		return fmt.Errorf("failed to parse JSON-RPC request: %w", err)
	}

	// Handle the request using the wrapped server
	response := s.server.Request(ctx, request)

	// Send the response
	if err := s.writeResponse(response); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	return nil
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
