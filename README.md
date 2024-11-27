# MCP-Go

A Go implementation of the Model Context Protocol (MCP), enabling seamless integration between LLM applications and external data sources and tools.

## About MCP

The Model Context Protocol (MCP) is an open protocol that enables seamless integration between LLM applications and external data sources and tools.
Learn more at [modelcontextprotocol.io](https://modelcontextprotocol.io/) and view the specification at [spec.modelcontextprotocol.io](https://spec.modelcontextprotocol.io/).

## Installation

```bash
go get github.com/mark3labs/mcp-go
```

## Features

- Standard IO (stdio) transport for communicating with child processes
- Server-Sent Events (SSE) transport for client-server architectures

## TODO
- Implement server package
- Other stuff???


## Examples

See the examples /examples directory for implementation examples including:

- Filesystem MCP server integration using stdio transport
- More examples coming soon...

## Contributing

I'm not an expert and this is my first Go library, so contributions are very welcome! Whether it's:

- Improving the code quality
- Adding features
- Fixing bugs
- Writing documentation
- Adding examples

Feel free to open issues and PRs. Let's make this library better together.

## License

MIT License
