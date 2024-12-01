package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/mark3labs/mcp-go/mcp"
)

type SSEMCPClient struct {
	baseURL     *url.URL
	endpoint    *url.URL
	httpClient  *http.Client
	requestID   atomic.Int64
	responses   map[int64]chan *json.RawMessage
	mu          sync.RWMutex
	done        chan struct{}
	initialized bool
}

func NewSSEMCPClient(baseURL string) (*SSEMCPClient, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	return &SSEMCPClient{
		baseURL:    parsedURL,
		httpClient: &http.Client{},
		responses:  make(map[int64]chan *json.RawMessage),
		done:       make(chan struct{}),
	}, nil
}

func (c *SSEMCPClient) Start(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE stream: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	go c.readSSE(resp.Body)
	return nil
}

func (c *SSEMCPClient) readSSE(reader io.ReadCloser) {
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	var event, data string

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Empty line means end of event
			if event != "" && data != "" {
				c.handleSSEEvent(event, data)
				event = ""
				data = ""
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-c.done:
			// Client is closing, ignore error
			return
		default:
			fmt.Printf("SSE stream error: %v\n", err)
		}
	}
}

func (c *SSEMCPClient) handleSSEEvent(event, data string) {
	switch event {
	case "endpoint":
		endpoint, err := url.Parse(data)
		if err != nil {
			fmt.Printf("Error parsing endpoint URL: %v\n", err)
			return
		}
		if endpoint.Host != c.baseURL.Host {
			fmt.Printf("Endpoint origin does not match connection origin\n")
			return
		}
		c.endpoint = endpoint

	case "message":
		var response struct {
			ID     int64           `json:"id"`
			Result json.RawMessage `json:"result,omitempty"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error,omitempty"`
		}

		if err := json.Unmarshal([]byte(data), &response); err != nil {
			fmt.Printf("Error unmarshaling response: %v\n", err)
			return
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

func (c *SSEMCPClient) sendRequest(
	ctx context.Context,
	method string,
	params interface{},
) (*json.RawMessage, error) {
	if !c.initialized && method != "initialize" {
		return nil, fmt.Errorf("client not initialized")
	}

	if c.endpoint == nil {
		return nil, fmt.Errorf("endpoint not received")
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

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.endpoint.String(),
		bytes.NewReader(requestBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Accept both 200 OK and 202 Accepted as valid responses
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"request failed with status %d: %s",
			resp.StatusCode,
			body,
		)
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

func (c *SSEMCPClient) Initialize(
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

func (c *SSEMCPClient) Ping(ctx context.Context) error {
	_, err := c.sendRequest(ctx, "ping", nil)
	return err
}

func (c *SSEMCPClient) ListResources(
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

func (c *SSEMCPClient) ReadResource(
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

func (c *SSEMCPClient) Subscribe(ctx context.Context, uri string) error {
	params := struct {
		URI string `json:"uri"`
	}{
		URI: uri,
	}

	_, err := c.sendRequest(ctx, "resources/subscribe", params)
	return err
}

func (c *SSEMCPClient) Unsubscribe(ctx context.Context, uri string) error {
	params := struct {
		URI string `json:"uri"`
	}{
		URI: uri,
	}

	_, err := c.sendRequest(ctx, "resources/unsubscribe", params)
	return err
}

func (c *SSEMCPClient) ListPrompts(
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

func (c *SSEMCPClient) GetPrompt(
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

func (c *SSEMCPClient) ListTools(
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

func (c *SSEMCPClient) CallTool(
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

func (c *SSEMCPClient) SetLevel(
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

func (c *SSEMCPClient) Complete(
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

func (c *SSEMCPClient) GetEndpoint() *url.URL {
	return c.endpoint
}

func (c *SSEMCPClient) Close() error {
	select {
	case <-c.done:
		return nil // Already closed
	default:
		close(c.done)
	}

	// Clean up any pending responses
	c.mu.Lock()
	for _, ch := range c.responses {
		close(ch)
	}
	c.responses = make(map[int64]chan *json.RawMessage)
	c.mu.Unlock()

	return nil
}
