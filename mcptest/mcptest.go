// Package mcptest implements helper functions for testing MCP servers.
package mcptest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"testing"

	"github.com/zillow/mcp-go/client"
	"github.com/zillow/mcp-go/client/transport"
	"github.com/zillow/mcp-go/mcp"
	"github.com/zillow/mcp-go/server"
)

// Server encapsulates an MCP server and manages resources like pipes and context.
type Server struct {
	name  string
	tools []server.ServerTool

	ctx    context.Context
	cancel func()

	serverReader *io.PipeReader
	serverWriter *io.PipeWriter
	clientReader *io.PipeReader
	clientWriter *io.PipeWriter

	logBuffer bytes.Buffer

	transport transport.Interface
	client    *client.Client

	wg sync.WaitGroup
}

// NewServer starts a new MCP server with the provided tools and returns the server instance.
func NewServer(t *testing.T, tools ...server.ServerTool) (*Server, error) {
	server := NewUnstartedServer(t)
	server.AddTools(tools...)

	if err := server.Start(); err != nil {
		return nil, err
	}

	return server, nil
}

// NewUnstartedServer creates a new MCP server instance with the given name, but does not start the server.
// Useful for tests where you need to add tools before starting the server.
func NewUnstartedServer(t *testing.T) *Server {
	server := &Server{
		name: t.Name(),
	}

	// Use t.Context() once we switch to go >= 1.24
	ctx := context.TODO()

	// Set up context with cancellation, used to stop the server
	server.ctx, server.cancel = context.WithCancel(ctx)

	// Set up pipes for client-server communication
	server.serverReader, server.clientWriter = io.Pipe()
	server.clientReader, server.serverWriter = io.Pipe()

	// Return the configured server
	return server
}

// AddTools adds multiple tools to an unstarted server.
func (s *Server) AddTools(tools ...server.ServerTool) {
	s.tools = append(s.tools, tools...)
}

// AddTool adds a tool to an unstarted server.
func (s *Server) AddTool(tool mcp.Tool, handler server.ToolHandlerFunc) {
	s.tools = append(s.tools, server.ServerTool{
		Tool:    tool,
		Handler: handler,
	})
}

// Start starts the server in a goroutine. Make sure to defer Close() after Start().
// When using NewServer(), the returned server is already started.
func (s *Server) Start() error {
	s.wg.Add(1)

	// Start the MCP server in a goroutine
	go func() {
		defer s.wg.Done()

		mcpServer := server.NewMCPServer(s.name, "1.0.0")

		mcpServer.AddTools(s.tools...)

		logger := log.New(&s.logBuffer, "", 0)

		stdioServer := server.NewStdioServer(mcpServer)
		stdioServer.SetErrorLogger(logger)

		if err := stdioServer.Listen(s.ctx, s.serverReader, s.serverWriter); err != nil {
			logger.Println("StdioServer.Listen failed:", err)
		}
	}()

	s.transport = transport.NewIO(s.clientReader, s.clientWriter, io.NopCloser(&s.logBuffer))
	if err := s.transport.Start(s.ctx); err != nil {
		return fmt.Errorf("transport.Start(): %w", err)
	}

	s.client = client.NewClient(s.transport)

	var initReq mcp.InitializeRequest
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	if _, err := s.client.Initialize(s.ctx, initReq); err != nil {
		return fmt.Errorf("client.Initialize(): %w", err)
	}

	return nil
}

// Close stops the server and cleans up resources like temporary directories.
func (s *Server) Close() {
	if s.transport != nil {
		s.transport.Close()
		s.transport = nil
		s.client = nil
	}

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	// Wait for server goroutine to finish
	s.wg.Wait()

	s.serverWriter.Close()
	s.serverReader.Close()
	s.serverReader, s.serverWriter = nil, nil

	s.clientWriter.Close()
	s.clientReader.Close()
	s.clientReader, s.clientWriter = nil, nil
}

// Client returns an MCP client connected to the server.
// The client is already initialized, i.e. you do _not_ need to call Client.Initialize().
func (s *Server) Client() *client.Client {
	return s.client
}
