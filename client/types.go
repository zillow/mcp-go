package client

// Supporting types needed for the interface
type ClientCapabilities struct {
	Experimental map[string]map[string]interface{} `json:"experimental,omitempty"`
	Roots        *struct {
		ListChanged bool `json:"listChanged"`
	} `json:"roots,omitempty"`
	Sampling map[string]interface{} `json:"sampling,omitempty"`
}

type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type CompleteArgument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type LoggingLevel string

const (
	LoggingLevelEmergency LoggingLevel = "emergency"
	LoggingLevelAlert     LoggingLevel = "alert"
	LoggingLevelCritical  LoggingLevel = "critical"
	LoggingLevelError     LoggingLevel = "error"
	LoggingLevelWarning   LoggingLevel = "warning"
	LoggingLevelNotice    LoggingLevel = "notice"
	LoggingLevelInfo      LoggingLevel = "info"
	LoggingLevelDebug     LoggingLevel = "debug"
)

// Result types
type InitializeResult struct {
	Meta            *MetaData          `json:"_meta,omitempty"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	Instructions    string             `json:"instructions,omitempty"`
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      Implementation     `json:"serverInfo"`
}

type ListResourcesResult struct {
	Meta       *MetaData  `json:"_meta,omitempty"`
	NextCursor string     `json:"nextCursor,omitempty"`
	Resources  []Resource `json:"resources"`
}

type ReadResourceResult struct {
	Meta     *MetaData          `json:"_meta,omitempty"`
	Contents []ResourceContents `json:"contents"`
}

type ListPromptsResult struct {
	Meta       *MetaData `json:"_meta,omitempty"`
	NextCursor string    `json:"nextCursor,omitempty"`
	Prompts    []Prompt  `json:"prompts"`
}

type GetPromptResult struct {
	Meta        *MetaData       `json:"_meta,omitempty"`
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

type ListToolsResult struct {
	Meta       *MetaData `json:"_meta,omitempty"`
	NextCursor string    `json:"nextCursor,omitempty"`
	Tools      []Tool    `json:"tools"`
}

type CallToolResult struct {
	Meta    *MetaData `json:"_meta,omitempty"`
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type CompleteResult struct {
	Meta       *MetaData  `json:"_meta,omitempty"`
	Completion Completion `json:"completion"`
}

// Supporting types
type MetaData map[string]interface{}

type ServerCapabilities struct {
	Experimental map[string]map[string]interface{} `json:"experimental,omitempty"`
	Logging      map[string]interface{}            `json:"logging,omitempty"`
	Prompts      *struct {
		ListChanged bool `json:"listChanged"`
	} `json:"prompts,omitempty"`
	Resources *struct {
		ListChanged bool `json:"listChanged"`
		Subscribe   bool `json:"subscribe"`
	} `json:"resources,omitempty"`
	Tools *struct {
		ListChanged bool `json:"listChanged"`
	} `json:"tools,omitempty"`
}

type Resource struct {
	Annotations *Annotations `json:"annotations,omitempty"`
	Description string       `json:"description,omitempty"`
	MimeType    string       `json:"mimeType,omitempty"`
	Name        string       `json:"name"`
	URI         string       `json:"uri"`
}

type ResourceContents interface{} // Can be either TextResourceContents or BlobResourceContents

type TextResourceContents struct {
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text"`
	URI      string `json:"uri"`
}

type BlobResourceContents struct {
	Blob     []byte `json:"blob"`
	MimeType string `json:"mimeType,omitempty"`
	URI      string `json:"uri"`
}

type Prompt struct {
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	Description string           `json:"description,omitempty"`
	Name        string           `json:"name"`
}

type PromptArgument struct {
	Description string `json:"description,omitempty"`
	Name        string `json:"name"`
	Required    bool   `json:"required,omitempty"`
}

type PromptMessage struct {
	Content Content `json:"content"`
	Role    Role    `json:"role"`
}

type Content interface{} // Can be TextContent, ImageContent, or EmbeddedResource

type TextContent struct {
	Annotations *Annotations `json:"annotations,omitempty"`
	Text        string       `json:"text"`
	Type        string       `json:"type"` // Always "text"
}

type ImageContent struct {
	Annotations *Annotations `json:"annotations,omitempty"`
	Data        []byte       `json:"data"`
	MimeType    string       `json:"mimeType"`
	Type        string       `json:"type"` // Always "image"
}

type EmbeddedResource struct {
	Annotations *Annotations     `json:"annotations,omitempty"`
	Resource    ResourceContents `json:"resource"`
	Type        string           `json:"type"` // Always "resource"
}

type Tool struct {
	Description string          `json:"description,omitempty"`
	InputSchema ToolInputSchema `json:"inputSchema"`
	Name        string          `json:"name"`
}

type ToolInputSchema struct {
	Properties map[string]interface{} `json:"properties,omitempty"`
	Type       string                 `json:"type"` // Always "object"
}

type Completion struct {
	HasMore bool     `json:"hasMore,omitempty"`
	Total   int      `json:"total,omitempty"`
	Values  []string `json:"values"`
}

type Annotations struct {
	Audience []Role  `json:"audience,omitempty"`
	Priority float64 `json:"priority,omitempty"`
}

type Role string

const (
	RoleAssistant Role = "assistant"
	RoleUser      Role = "user"
)

// Reference types
type PromptReference struct {
	Type string `json:"type"` // const "ref/prompt"
	Name string `json:"name"`
}

type ResourceReference struct {
	Type string `json:"type"` // const "ref/resource"
	URI  string `json:"uri"`
}
