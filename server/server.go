package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

type MCPServer interface {
	Request(ctx context.Context, request mcp.JSONRPCRequest) mcp.JSONRPCResponse
	RequestSampling(
		ctx context.Context,
		request mcp.CreateMessageRequest,
	) mcp.JSONRPCRequest
	HandleInitialize(InitializeFunc)
	HandlePing(PingFunc)
	HandleListResources(ListResourcesFunc)
	HandleListResourceTemplates(ListResourceTemplatesFunc)
	HandleReadResource(ReadResourceFunc)
	HandleSubscribe(SubscribeFunc)
	HandleUnsubscribe(UnsubscribeFunc)
	HandleListPrompts(ListPromptsFunc)
	HandleGetPrompt(GetPromptFunc)
	HandleListTools(ListToolsFunc)
	HandleCallTool(CallToolFunc)
	HandleSetLevel(SetLevelFunc)
	HandleComplete(CompleteFunc)
	HandleNotification(string, NotificationFunc)
}

type DefaultServer struct {
	handlers map[string]interface{}
	name     string
	version  string
}

func NewDefaultServer(name, version string) MCPServer {
	s := &DefaultServer{
		handlers: make(map[string]interface{}),
		name:     name,
		version:  version,
	}

	s.HandleInitialize(s.defaultInitialize)
	s.HandlePing(s.defaultPing)
	s.HandleListResources(s.defaultListResources)
	s.HandleListResourceTemplates(s.defaultListResourceTemplates)
	s.HandleReadResource(s.defaultReadResource)
	s.HandleSubscribe(s.defaultSubscribe)
	s.HandleUnsubscribe(s.defaultUnsubscribe)
	s.HandleListPrompts(s.defaultListPrompts)
	s.HandleGetPrompt(s.defaultGetPrompt)
	s.HandleListTools(s.defaultListTools)
	s.HandleCallTool(s.defaultCallTool)
	s.HandleSetLevel(s.defaultSetLevel)
	s.HandleComplete(s.defaultComplete)

	return s
}

func (s *DefaultServer) Request(
	ctx context.Context,
	request mcp.JSONRPCRequest,
) mcp.JSONRPCResponse {
	handler, ok := s.handlers[request.Method]
	if !ok {
		return mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPC_VERSION,
			ID:      request.ID,
			Error: struct {
				Code    int         `json:"code"`
				Message string      `json:"message"`
				Data    interface{} `json:"data,omitempty"`
			}{
				Code:    mcp.METHOD_NOT_FOUND,
				Message: "Method not found",
			},
		}
	}

	result, err := callHandler(ctx, handler, request.Params)
	if err != nil {
		return mcp.JSONRPCResponse{
			JSONRPC: mcp.JSONRPC_VERSION,
			ID:      request.ID,
			Error: struct {
				Code    int         `json:"code"`
				Message string      `json:"message"`
				Data    interface{} `json:"data,omitempty"`
			}{
				Code:    mcp.INTERNAL_ERROR,
				Message: err.Error(),
			},
		}
	}

	return mcp.JSONRPCResponse{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      request.ID,
		Result:  result,
	}
}

func (s *DefaultServer) RequestSampling(
	ctx context.Context,
	request mcp.CreateMessageRequest,
) mcp.JSONRPCRequest {
	// Implementation for requesting sampling
	panic("Not implemented")
}

// Handler type definitions
type InitializeFunc func(context.Context, mcp.InitializeRequest) (mcp.InitializeResult, error)
type PingFunc func(context.Context, mcp.PingRequest) (mcp.EmptyResult, error)

type ListResourcesFunc func(context.Context, mcp.ListResourcesRequest) (mcp.ListResourcesResult, error)

type ListResourceTemplatesFunc func(context.Context, mcp.ListResourceTemplatesRequest) (mcp.ListResourceTemplatesResult, error)

type ReadResourceFunc func(context.Context, mcp.ReadResourceRequest) (mcp.ReadResourceResult, error)

type SubscribeFunc func(context.Context, mcp.SubscribeRequest) (mcp.EmptyResult, error)

type UnsubscribeFunc func(context.Context, mcp.UnsubscribeRequest) (mcp.EmptyResult, error)

type ListPromptsFunc func(context.Context, mcp.ListPromptsRequest) (mcp.ListPromptsResult, error)

type GetPromptFunc func(context.Context, mcp.GetPromptRequest) (mcp.GetPromptResult, error)

type ListToolsFunc func(context.Context, mcp.ListToolsRequest) (mcp.ListToolsResult, error)

type CallToolFunc func(context.Context, mcp.CallToolRequest) (mcp.CallToolResult, error)

type SetLevelFunc func(context.Context, mcp.SetLevelRequest) (mcp.EmptyResult, error)

type CompleteFunc func(context.Context, mcp.CompleteRequest) (mcp.CompleteResult, error)
type NotificationFunc func(context.Context, mcp.JSONRPCNotification)

// Handler registration methods
func (s *DefaultServer) HandleInitialize(
	f InitializeFunc,
) {
	s.handlers["initialize"] = f
}

func (s *DefaultServer) HandlePing(
	f PingFunc,
) {
	s.handlers["ping"] = f
}

func (s *DefaultServer) HandleListResources(
	f ListResourcesFunc,
) {
	s.handlers["resources/list"] = f
}

func (s *DefaultServer) HandleListResourceTemplates(
	f ListResourceTemplatesFunc,
) {
	s.handlers["resources/templates/list"] = f
}

func (s *DefaultServer) HandleReadResource(
	f ReadResourceFunc,
) {
	s.handlers["resources/read"] = f
}

func (s *DefaultServer) HandleSubscribe(
	f SubscribeFunc,
) {
	s.handlers["resources/subscribe"] = f
}

func (s *DefaultServer) HandleUnsubscribe(
	f UnsubscribeFunc,
) {
	s.handlers["resources/unsubscribe"] = f
}

func (s *DefaultServer) HandleListPrompts(
	f ListPromptsFunc,
) {
	s.handlers["prompts/list"] = f
}

func (s *DefaultServer) HandleGetPrompt(
	f GetPromptFunc,
) {
	s.handlers["prompts/get"] = f
}

func (s *DefaultServer) HandleListTools(
	f ListToolsFunc,
) {
	s.handlers["tools/list"] = f
}

func (s *DefaultServer) HandleCallTool(
	f CallToolFunc,
) {
	s.handlers["tools/call"] = f
}

func (s *DefaultServer) HandleSetLevel(
	f SetLevelFunc,
) {
	s.handlers["logging/setLevel"] = f
}

func (s *DefaultServer) HandleComplete(
	f CompleteFunc,
) {
	s.handlers["completion/complete"] = f
}

func (s *DefaultServer) HandleNotification(
	method string,
	f NotificationFunc,
) {
	s.handlers[method] = f
}

// Default handler implementations
func (s *DefaultServer) defaultInitialize(
	ctx context.Context,
	req mcp.InitializeRequest,
) (mcp.InitializeResult, error) {
	return mcp.InitializeResult{
		ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
		ServerInfo: mcp.Implementation{
			Name:    s.name,
			Version: s.version,
		},
		Capabilities: mcp.ServerCapabilities{},
	}, nil
}

func (s *DefaultServer) defaultPing(
	ctx context.Context,
	req mcp.PingRequest,
) (mcp.EmptyResult, error) {
	return mcp.EmptyResult{}, nil
}

func (s *DefaultServer) defaultListResources(
	ctx context.Context,
	req mcp.ListResourcesRequest,
) (mcp.ListResourcesResult, error) {
	return mcp.ListResourcesResult{Resources: []mcp.Resource{}}, nil
}

func (s *DefaultServer) defaultListResourceTemplates(
	ctx context.Context,
	req mcp.ListResourceTemplatesRequest,
) (mcp.ListResourceTemplatesResult, error) {
	return mcp.ListResourceTemplatesResult{
		ResourceTemplates: []mcp.ResourceTemplate{},
	}, nil
}

func (s *DefaultServer) defaultReadResource(
	ctx context.Context,
	req mcp.ReadResourceRequest,
) (mcp.ReadResourceResult, error) {
	return mcp.ReadResourceResult{}, fmt.Errorf("Resource not found")
}

func (s *DefaultServer) defaultSubscribe(
	ctx context.Context,
	req mcp.SubscribeRequest,
) (mcp.EmptyResult, error) {
	return mcp.EmptyResult{}, nil
}

func (s *DefaultServer) defaultUnsubscribe(
	ctx context.Context,
	req mcp.UnsubscribeRequest,
) (mcp.EmptyResult, error) {
	return mcp.EmptyResult{}, nil
}

func (s *DefaultServer) defaultListPrompts(
	ctx context.Context,
	req mcp.ListPromptsRequest,
) (mcp.ListPromptsResult, error) {
	return mcp.ListPromptsResult{Prompts: []mcp.Prompt{}}, nil
}

func (s *DefaultServer) defaultGetPrompt(
	ctx context.Context,
	req mcp.GetPromptRequest,
) (mcp.GetPromptResult, error) {
	return mcp.GetPromptResult{}, fmt.Errorf("Prompt not found")
}

func (s *DefaultServer) defaultListTools(
	ctx context.Context,
	req mcp.ListToolsRequest,
) (mcp.ListToolsResult, error) {
	return mcp.ListToolsResult{Tools: []mcp.Tool{}}, nil
}

func (s *DefaultServer) defaultCallTool(
	ctx context.Context,
	req mcp.CallToolRequest,
) (mcp.CallToolResult, error) {
	return mcp.CallToolResult{}, fmt.Errorf("Tool not found")
}

func (s *DefaultServer) defaultSetLevel(
	ctx context.Context,
	req mcp.SetLevelRequest,
) (mcp.EmptyResult, error) {
	return mcp.EmptyResult{}, nil
}

func (s *DefaultServer) defaultComplete(
	ctx context.Context,
	req mcp.CompleteRequest,
) (mcp.CompleteResult, error) {
	return mcp.CompleteResult{}, nil
}

// Helper function to call handlers with proper type assertions
func callHandler(
	ctx context.Context,
	handler interface{},
	params interface{},
) (mcp.Result, error) {

	handlerType := fmt.Sprintf("%T", handler)
	var err error

	var result interface{}
	switch handlerType {
	case "server.InitializeFunc":
		if req, ok := params.(mcp.InitializeRequest); ok {
			result, err = handler.(InitializeFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for InitializeFunc")
		}

	case "server.PingFunc":
		if req, ok := params.(mcp.PingRequest); ok {
			result, err = handler.(PingFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for PingFunc")
		}

	case "server.ListResourcesFunc":
		if req, ok := params.(mcp.ListResourcesRequest); ok {
			result, err = handler.(ListResourcesFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for ListResourcesFunc")
		}

	case "server.ListResourceTemplatesFunc":
		if req, ok := params.(mcp.ListResourceTemplatesRequest); ok {
			result, err = handler.(ListResourceTemplatesFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for ListResourceTemplatesFunc")
		}
	case "server.ReadResourceFunc":
		if req, ok := params.(mcp.ReadResourceRequest); ok {
			result, err = handler.(ReadResourceFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for ReadResourceFunc")
		}
	case "server.SubscribeFunc":
		if req, ok := params.(mcp.SubscribeRequest); ok {
			result, err = handler.(SubscribeFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for SubscribeFunc")
		}
	case "server.UnsubscribeFunc":
		if req, ok := params.(mcp.UnsubscribeRequest); ok {
			result, err = handler.(UnsubscribeFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for UnsubscribeFunc")
		}
	case "server.ListPromptsFunc":
		if req, ok := params.(mcp.ListPromptsRequest); ok {
			result, err = handler.(ListPromptsFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for ListPromptsFunc")
		}
	case "server.GetPromptFunc":
		if req, ok := params.(mcp.GetPromptRequest); ok {
			result, err = handler.(GetPromptFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for GetPromptFunc")
		}
	case "server.ListToolsFunc":
		if req, ok := params.(mcp.ListToolsRequest); ok {
			result, err = handler.(ListToolsFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for ListToolsFunc")
		}
	case "server.CallToolFunc":
		if req, ok := params.(mcp.CallToolRequest); ok {
			result, err = handler.(CallToolFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for CallToolFunc")
		}
	case "server.SetLevelFunc":
		if req, ok := params.(mcp.SetLevelRequest); ok {
			result, err = handler.(SetLevelFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for SetLevelFunc")
		}
	case "server.CompleteFunc":
		if req, ok := params.(mcp.CompleteRequest); ok {
			result, err = handler.(CompleteFunc)(ctx, req)
		} else {
			err = fmt.Errorf("Invalid params for CompleteFunc")
		}
	default:
		return mcp.Result{}, fmt.Errorf("Unknown handler type: %s", handlerType)
	}

	if err != nil {
		return mcp.Result{}, err
	}

	// Wrap the result in mcp.Result if it's not already
	if res, ok := result.(mcp.Result); ok {
		return res, nil
	}
	return mcp.Result{
		Meta: map[string]interface{}{
			"data": result,
		},
	}, nil
}
