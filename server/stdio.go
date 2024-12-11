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

	"github.com/mark3labs/mcp-go/mcp"
)

// StdioServer wraps a MCPServer and handles stdio communication
// StdioServer wraps a MCPServer and handles stdio communication.
// It provides a simple way to create command-line MCP servers that
// communicate via standard input/output streams.
type StdioServer struct {
	server    *MCPServer
	errLogger *log.Logger
}

// NewStdioServer creates a new stdio server wrapper around an MCPServer
func NewStdioServer(server *MCPServer) *StdioServer {
	return &StdioServer{
		server:    server,
		errLogger: log.New(io.Discard, "", log.LstdFlags), // Default to discarding logs
	}
}

// SetErrorLogger allows configuring where errors are logged
func (s *StdioServer) SetErrorLogger(logger *log.Logger) {
	s.errLogger = logger
}

// Listen starts listening for messages on the provided input and writes responses to the provided output
func (s *StdioServer) Listen(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	reader := bufio.NewReader(stdin)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
				return ctx.Err()
			case err := <-errChan:
				if err == io.EOF {
					return nil
				}
				s.errLogger.Printf("Error reading input: %v", err)
				return err
			case line := <-readChan:
				if err := s.processMessage(ctx, line, stdout); err != nil {
					if err == io.EOF {
						return nil
					}
					s.errLogger.Printf("Error handling message: %v", err)
					return err
				}
			}
		}
	}
}

// processMessage handles a single message and writes the response
func (s *StdioServer) processMessage(ctx context.Context, line string, writer io.Writer) error {
	// Parse the message as raw JSON
	var rawMessage json.RawMessage
	if err := json.Unmarshal([]byte(line), &rawMessage); err != nil {
		response := createErrorResponse(nil, mcp.PARSE_ERROR, "Parse error")
		return s.writeResponse(response, writer)
	}

	// Handle the message using the wrapped server
	response := s.server.HandleMessage(ctx, rawMessage)

	// Send the response if there is one (notifications don't have responses)
	if response != nil {
		if err := s.writeResponse(response, writer); err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}
	}

	return nil
}

func (s *StdioServer) writeResponse(response mcp.JSONRPCMessage, writer io.Writer) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return err
	}

	// Write response followed by newline
	if _, err := fmt.Fprintf(writer, "%s\n", responseBytes); err != nil {
		return err
	}

	return nil
}

// ServeStdio is a convenience function that creates and starts a StdioServer with os.Stdin and os.Stdout
func ServeStdio(server *MCPServer) error {
	s := NewStdioServer(server)
	s.SetErrorLogger(log.New(os.Stderr, "", log.LstdFlags))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigChan
		cancel()
	}()

	return s.Listen(ctx, os.Stdin, os.Stdout)
}
