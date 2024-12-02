<!-- omit in toc -->
# MCP Go 🚀

<div align="center">

<strong>A Go implementation of the Model Context Protocol (MCP), enabling seamless integration between LLM applications and external data sources and tools.</strong>

</div>

```go
package main

import (
    "context"
    "fmt"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

func main() {
    mcp := server.NewMCPServer(
        "Demo 🚀",
        "1.0.0",
        server.WithToolCapabilities(true),
    )

    mcp.AddTool(mcp.Tool{
        Name:        "add",
        Description: "Add two numbers",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]interface{}{
                "a": map[string]interface{}{
                    "type":        "number",
                    "description": "First number",
                },
                "b": map[string]interface{}{
                    "type":        "number",
                    "description": "Second number",
                },
            },
        },
    }, addHandler)

    if err := server.ServeStdio(mcp); err != nil {
        fmt.Printf("Server error: %v\n", err)
    }
}

func addHandler(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
    a, ok1 := arguments["a"].(float64)
    b, ok2 := arguments["b"].(float64)
    if !ok1 || !ok2 {
        return nil, fmt.Errorf("invalid number arguments")
    }

    sum := int(a) + int(b)

    return &mcp.CallToolResult{
        Content: []interface{}{
            mcp.TextContent{
                Type: "text",
                Text: fmt.Sprintf("The sum of %d and %d is %d", int(a), int(b), sum),
            },
        },
    }, nil
}
```

That's it!

MCP Go handles all the complex protocol details and server management, so you can focus on building great tools. It aims to be high-level and easy to use.

### Key features:
* **Fast**: High-level interface means less code and faster development
* **Simple**: Build MCP servers with minimal boilerplate
* **Complete***: MCP Go aims to provide a full implementation of the core MCP specification

(\*emphasis on *aims*)

🚨 🚧 🏗️ *MCP Go is under active development, as is the MCP specification itself. Core features are working but some advanced capabilities are still in progress.* 


<!-- omit in toc -->
## Table of Contents

- [Installation](#installation)
- [Quickstart](#quickstart)
- [What is MCP?](#what-is-mcp)
- [Core Concepts](#core-concepts)
  - [Server](#server)
  - [Resources](#resources)
  - [Tools](#tools)
  - [Prompts](#prompts)
- [Examples](#examples)
- [Contributing](#contributing)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation-1)
  - [Testing](#testing)
  - [Opening a Pull Request](#opening-a-pull-request)

## Installation

```bash
go get github.com/mark3labs/mcp-go
```

## Quickstart

Let's create a simple MCP server that exposes a calculator tool and some data:

```go
// TODO
```
## What is MCP?

The [Model Context Protocol (MCP)](https://modelcontextprotocol.io) lets you build servers that expose data and functionality to LLM applications in a secure, standardized way. Think of it like a web API, but specifically designed for LLM interactions. MCP servers can:

- Expose data through **Resources** (think of these sort of like GET endpoints; they are used to load information into the LLM's context)
- Provide functionality through **Tools** (sort of like POST endpoints; they are used to execute code or otherwise produce a side effect)
- Define interaction patterns through **Prompts** (reusable templates for LLM interactions)
- And more!


## Core Concepts


### Server

The server is your core interface to the MCP protocol. It handles connection management, protocol compliance, and message routing:

```go
// TODO
```

### Resources

Resources are how you expose data to LLMs. They're similar to GET endpoints in a REST API - they provide data but shouldn't perform significant computation or have side effects. Some examples:

- File contents
- Database schemas
- API responses
- System information

Resources can be static:
```go
// TODO
```

### Tools

Tools let LLMs take actions through your server. Unlike resources, tools are expected to perform computation and have side effects. They're similar to POST endpoints in a REST API.

Simple calculation example:
```go
// TODO
```

HTTP request example:
```go
// TODO
```

### Prompts

Prompts are reusable templates that help LLMs interact with your server effectively. They're like "best practices" encoded into your server. A prompt can be as simple as a string:

```go
// TODO
```

Or a more structured sequence of messages:
```go
// TODO
```

## Examples

For examples, see the `examples/` directory.


## Contributing

<details>

<summary><h3>Open Developer Guide</h3></summary>

### Prerequisites

Go version >= 1.23

### Installation

Create a fork of this repository, then clone it:

```bash
git clone https://github.com/mark3labs/mcp-go.git
cd mcp-go
```

### Testing

Please make sure to test any new functionality. Your tests should be simple and atomic and anticipate change rather than cement complex patterns.

Run tests from the root directory:

```bash
go test -v './...'
```

### Opening a Pull Request

Fork the repository and create a new branch:

```bash
git checkout -b my-branch
```

Make your changes and commit them:


```bash
git add . && git commit -m "My changes"
```

Push your changes to your fork:


```bash
git push origin my-branch
```

Feel free to reach out in a GitHub issue or discussion if you have any questions!

</details>
