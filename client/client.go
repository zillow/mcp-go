package client

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Client represents an MCP client
type Client struct {
	config     ClientConfig
	transport  Transport
	nextReqID  atomic.Int64
	handlers   map[string]NotificationHandler
	handlersmu sync.RWMutex
}

// Transport defines the interface for different transport implementations
type Transport interface {
	Connect(context.Context) error
	Disconnect() error
	Send(context.Context, *JSONRPCMessage) error
	Receive(context.Context) (*JSONRPCMessage, error)
}

// NotificationHandler represents a handler for incoming notifications
type NotificationHandler func(context.Context, *JSONRPCMessage) error

// New creates a new MCP client
func New(config ClientConfig, transport Transport) *Client {
	return &Client{
		config:    config,
		transport: transport,
		handlers:  make(map[string]NotificationHandler),
	}
}

// Connect establishes a connection with the server
func (c *Client) Connect(ctx context.Context) error {
	if err := c.transport.Connect(ctx); err != nil {
		return err
	}

	// Send initialize request
	initReq := &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"clientInfo":      c.config.Implementation,
			"capabilities":    c.config.Capabilities,
			"protocolVersion": "1.0", // Update with actual version
		},
	}

	if err := c.transport.Send(ctx, initReq); err != nil {
		return err
	}

	// Wait for initialize response
	resp, err := c.transport.Receive(ctx)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return errors.New(resp.Error.Message)
	}

	// Send initialized notification
	initNotif := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}

	return c.transport.Send(ctx, initNotif)
}

// RegisterNotificationHandler registers a handler for a specific notification method
func (c *Client) RegisterNotificationHandler(
	method string,
	handler NotificationHandler,
) {
	c.handlersmu.Lock()
	defer c.handlersmu.Unlock()
	c.handlers[method] = handler
}

// nextRequestID generates the next request ID
func (c *Client) nextRequestID() RequestID {
	return c.nextReqID.Add(1)
}

// handleNotification handles incoming notifications
func (c *Client) handleNotification(
	ctx context.Context,
	msg *JSONRPCMessage,
) error {
	c.handlersmu.RLock()
	handler, ok := c.handlers[msg.Method]
	c.handlersmu.RUnlock()

	if !ok {
		return nil // Ignore unhandled notifications
	}

	return handler(ctx, msg)
}
