package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
)

type StdioMCPClient struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      *bufio.Reader
	requestID   atomic.Int64
	responses   map[int64]chan *json.RawMessage
	mu          sync.RWMutex
	done        chan struct{}
	initialized bool
}

func NewStdioMCPClient(
	command string,
	args ...string,
) (*StdioMCPClient, error) {
	cmd := exec.Command(command, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	client := &StdioMCPClient{
		cmd:       cmd,
		stdin:     stdin,
		stdout:    bufio.NewReader(stdout),
		responses: make(map[int64]chan *json.RawMessage),
		done:      make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	go client.readResponses()

	return client, nil
}

func (c *StdioMCPClient) Close() error {
	close(c.done)
	if err := c.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	return c.cmd.Wait()
}

func (c *StdioMCPClient) readResponses() {
	for {
		select {
		case <-c.done:
			return
		default:
			line, err := c.stdout.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Error reading response: %v\n", err)
				}
				return
			}

			var response struct {
				ID     int64           `json:"id"`
				Result json.RawMessage `json:"result,omitempty"`
				Error  *struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				} `json:"error,omitempty"`
			}

			if err := json.Unmarshal([]byte(line), &response); err != nil {
				continue
			}

			c.mu.RLock()
			ch, ok := c.responses[response.ID]
			c.mu.RUnlock()

			if ok {
				if response.Error != nil {
					ch <- nil // Signal error condition
				} else {
					ch <- &response.Result
				}
				c.mu.Lock()
				delete(c.responses, response.ID)
				c.mu.Unlock()
			}
		}
	}
}

func (c *StdioMCPClient) sendRequest(
	ctx context.Context,
	method string,
	params interface{},
) (*json.RawMessage, error) {
	if !c.initialized && method != "initialize" {
		return nil, fmt.Errorf("client not initialized")
	}

	id := c.requestID.Add(1)

	request := struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      int64       `json:"id"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	responseChan := make(chan *json.RawMessage, 1)
	c.mu.Lock()
	c.responses[id] = responseChan
	c.mu.Unlock()

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	requestBytes = append(requestBytes, '\n')

	if _, err := c.stdin.Write(requestBytes); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.responses, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case response := <-responseChan:
		if response == nil {
			return nil, fmt.Errorf("request failed")
		}
		return response, nil
	}
}

func (c *StdioMCPClient) Initialize(
	ctx context.Context,
	capabilities mcp.ClientCapabilities,
	clientInfo mcp.Implementation,
	protocolVersion string,
) (*mcp.InitializeResult, error) {
	params := struct {
		Capabilities    mcp.ClientCapabilities `json:"capabilities"`
		ClientInfo      mcp.Implementation     `json:"clientInfo"`
		ProtocolVersion string                 `json:"protocolVersion"`
	}{
		Capabilities:    capabilities,
		ClientInfo:      clientInfo,
		ProtocolVersion: protocolVersion,
	}

	response, err := c.sendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}

	var result mcp.InitializeResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.initialized = true
	return &result, nil
}

func (c *StdioMCPClient) Ping(ctx context.Context) error {
	_, err := c.sendRequest(ctx, "ping", nil)
	return err
}

func (c *StdioMCPClient) ListResources(
	ctx context.Context,
	cursor *string,
) (*mcp.ListResourcesResult, error) {
	params := struct {
		Cursor *string `json:"cursor,omitempty"`
	}{
		Cursor: cursor,
	}

	response, err := c.sendRequest(ctx, "resources/list", params)
	if err != nil {
		return nil, err
	}

	var result mcp.ListResourcesResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) ReadResource(
	ctx context.Context,
	uri string,
) (*mcp.ReadResourceResult, error) {
	params := struct {
		URI string `json:"uri"`
	}{
		URI: uri,
	}

	response, err := c.sendRequest(ctx, "resources/read", params)
	if err != nil {
		return nil, err
	}

	var result mcp.ReadResourceResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) Subscribe(ctx context.Context, uri string) error {
	params := struct {
		URI string `json:"uri"`
	}{
		URI: uri,
	}

	_, err := c.sendRequest(ctx, "resources/subscribe", params)
	return err
}

func (c *StdioMCPClient) Unsubscribe(ctx context.Context, uri string) error {
	params := struct {
		URI string `json:"uri"`
	}{
		URI: uri,
	}

	_, err := c.sendRequest(ctx, "resources/unsubscribe", params)
	return err
}

func (c *StdioMCPClient) ListPrompts(
	ctx context.Context,
	cursor *string,
) (*mcp.ListPromptsResult, error) {
	params := struct {
		Cursor *string `json:"cursor,omitempty"`
	}{
		Cursor: cursor,
	}

	response, err := c.sendRequest(ctx, "prompts/list", params)
	if err != nil {
		return nil, err
	}

	var result mcp.ListPromptsResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) GetPrompt(
	ctx context.Context,
	name string,
	arguments map[string]string,
) (*mcp.GetPromptResult, error) {
	params := struct {
		Name      string            `json:"name"`
		Arguments map[string]string `json:"arguments,omitempty"`
	}{
		Name:      name,
		Arguments: arguments,
	}

	response, err := c.sendRequest(ctx, "prompts/get", params)
	if err != nil {
		return nil, err
	}

	var result mcp.GetPromptResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) ListTools(
	ctx context.Context,
	cursor *string,
) (*mcp.ListToolsResult, error) {
	params := struct {
		Cursor *string `json:"cursor,omitempty"`
	}{
		Cursor: cursor,
	}

	response, err := c.sendRequest(ctx, "tools/list", params)
	if err != nil {
		return nil, err
	}

	var result mcp.ListToolsResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) CallTool(
	ctx context.Context,
	name string,
	arguments map[string]interface{},
) (*mcp.CallToolResult, error) {
	params := struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments,omitempty"`
	}{
		Name:      name,
		Arguments: arguments,
	}

	response, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var result mcp.CallToolResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

func (c *StdioMCPClient) SetLevel(
	ctx context.Context,
	level mcp.LoggingLevel,
) error {
	params := struct {
		Level mcp.LoggingLevel `json:"level"`
	}{
		Level: level,
	}

	_, err := c.sendRequest(ctx, "logging/setLevel", params)
	return err
}

func (c *StdioMCPClient) Complete(
	ctx context.Context,
	ref interface{},
	argument mcp.CompleteRequestParamsArgument,
) (*mcp.CompleteResult, error) {
	params := struct {
		Ref      interface{}                       `json:"ref"`
		Argument mcp.CompleteRequestParamsArgument `json:"argument"`
	}{
		Ref:      ref,
		Argument: argument,
	}

	response, err := c.sendRequest(ctx, "completion/complete", params)
	if err != nil {
		return nil, err
	}

	var result mcp.CompleteResult
	if err := json.Unmarshal(*response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}
