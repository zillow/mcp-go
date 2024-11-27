package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// SSETransport implements the Transport interface using Server-Sent Events
type SSETransport struct {
	baseURL     string       // Base URL for the SSE endpoint
	postURL     string       // URL for sending messages (received in endpoint event)
	httpClient  *http.Client // HTTP client for making requests
	eventSource *EventSource // SSE event source
	mu          sync.RWMutex // protects postURL and connection state
	connected   bool         // connection state
}

// EventSource represents a connection to an SSE stream
type EventSource struct {
	url        string
	httpClient *http.Client
	resp       *http.Response
	scanner    *EventScanner
	events     chan SSEEvent
	errors     chan error
	done       chan struct{}
	closeOnce  sync.Once
}

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Type string
	Data string
}

// SSEOption represents an option for configuring the SSE transport
type SSEOption func(*SSETransport)

// WithHTTPClient sets a custom HTTP client for the SSE transport
func WithHTTPClient(client *http.Client) SSEOption {
	return func(t *SSETransport) {
		t.httpClient = client
	}
}

// NewSSETransport creates a new SSE transport
func NewSSETransport(baseURL string, opts ...SSEOption) *SSETransport {
	t := &SSETransport{
		baseURL:    baseURL,
		httpClient: http.DefaultClient,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// Connect establishes an SSE connection and waits for the endpoint event
func (t *SSETransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	es, err := newEventSource(ctx, t.baseURL, t.httpClient)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE endpoint: %w", err)
	}

	// Wait for endpoint event
	select {
	case evt := <-es.events:
		if evt.Type != "endpoint" {
			es.Close()
			return fmt.Errorf("expected endpoint event, got %s", evt.Type)
		}
		t.postURL = evt.Data
	case err := <-es.errors:
		es.Close()
		return fmt.Errorf("error waiting for endpoint: %w", err)
	case <-ctx.Done():
		es.Close()
		return ctx.Err()
	}

	t.eventSource = es
	t.connected = true
	return nil
}

// Disconnect closes the SSE connection
func (t *SSETransport) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil
	}

	if t.eventSource != nil {
		t.eventSource.Close()
		t.eventSource = nil
	}

	t.connected = false
	return nil
}

// Send sends a JSON-RPC message via HTTP POST
func (t *SSETransport) Send(ctx context.Context, msg *JSONRPCMessage) error {
	if msg == nil {
		return errors.New("message cannot be nil")
	}

	t.mu.RLock()
	if !t.connected {
		t.mu.RUnlock()
		return errors.New("not connected")
	}
	postURL := t.postURL
	t.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		postURL,
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"unexpected status code %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	return nil
}

// Receive receives a JSON-RPC message from the SSE stream
func (t *SSETransport) Receive(ctx context.Context) (*JSONRPCMessage, error) {
	t.mu.RLock()
	if !t.connected || t.eventSource == nil {
		t.mu.RUnlock()
		return nil, errors.New("not connected")
	}
	es := t.eventSource
	t.mu.RUnlock()

	select {
	case evt := <-es.events:
		if evt.Type != "message" {
			return nil, fmt.Errorf("unexpected event type: %s", evt.Type)
		}

		var msg JSONRPCMessage
		if err := json.Unmarshal([]byte(evt.Data), &msg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message: %w", err)
		}
		return &msg, nil

	case err := <-es.errors:
		return nil, fmt.Errorf("SSE error: %w", err)

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// newEventSource creates a new EventSource connection
func newEventSource(
	ctx context.Context,
	url string,
	client *http.Client,
) (*EventSource, error) {
	es := &EventSource{
		url:        url,
		httpClient: client,
		events: make(
			chan SSEEvent,
			10,
		), // Buffered channel to prevent blocking
		errors: make(chan error, 1),
		done:   make(chan struct{}),
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	es.resp = resp
	es.scanner = NewEventScanner(resp.Body)
	go es.readEvents()
	return es, nil
}

// readEvents reads events from the SSE stream
func (es *EventSource) readEvents() {
	defer es.resp.Body.Close()

	for es.scanner.Scan() {
		select {
		case <-es.done:
			return
		case es.events <- es.scanner.Event():
		}
	}

	if err := es.scanner.Err(); err != nil {
		select {
		case <-es.done:
		case es.errors <- err:
		}
	}
}

// Close closes the EventSource connection
func (es *EventSource) Close() error {
	es.closeOnce.Do(func() {
		close(es.done)
		if es.resp != nil && es.resp.Body != nil {
			es.resp.Body.Close()
		}
	})
	return nil
}

// EventScanner is a helper type for parsing SSE streams
type EventScanner struct {
	scanner *bufio.Scanner
	current SSEEvent
	err     error
}

// NewEventScanner creates a new EventScanner
func NewEventScanner(r io.Reader) *EventScanner {
	return &EventScanner{
		scanner: bufio.NewScanner(r),
	}
}

// Scan advances to the next event
func (s *EventScanner) Scan() bool {
	s.current = SSEEvent{}
	inEvent := false

	for s.scanner.Scan() {
		line := s.scanner.Text()

		// Empty line marks the end of an event
		if line == "" {
			if inEvent {
				return true
			}
			continue
		}

		inEvent = true

		// Parse the line
		if strings.HasPrefix(line, "event:") {
			s.current.Type = strings.TrimSpace(line[6:])
		} else if strings.HasPrefix(line, "data:") {
			if s.current.Data != "" {
				s.current.Data += "\n"
			}
			s.current.Data += strings.TrimSpace(line[5:])
		} else if strings.HasPrefix(line, ":") {
			// Comment line, ignore
			continue
		} else {
			// Malformed line, treat as part of current field if possible
			if strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				field := strings.TrimSpace(parts[0])
				value := ""
				if len(parts) > 1 {
					value = strings.TrimSpace(parts[1])
				}

				switch field {
				case "event":
					s.current.Type = value
				case "data":
					if s.current.Data != "" {
						s.current.Data += "\n"
					}
					s.current.Data += value
				}
			}
		}
	}

	// Return any partial event at EOF
	if inEvent {
		return true
	}

	s.err = s.scanner.Err()
	return false
}

// Event returns the current event
func (s *EventScanner) Event() SSEEvent {
	return s.current
}

// Err returns any error that occurred during scanning
func (s *EventScanner) Err() error {
	return s.err
}
