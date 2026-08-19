package mcp

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"
)

// Version is the MCP server version, set at build time.
var Version = "dev"

// NewServer creates a fully configured MCP server with all PinchTab tools registered.
func NewServer(baseURL, token string) *server.MCPServer {
	c := NewClient(baseURL, token)

	s := server.NewMCPServer(
		"PinchTab",
		Version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	tools := allTools()
	handlers := handlerMap(c)

	for _, tool := range tools {
		h, ok := handlers[tool.Name]
		if !ok {
			panic(fmt.Sprintf("mcp: no handler for tool %q", tool.Name))
		}
		s.AddTool(tool, h)
	}

	return s
}

// Serve starts the MCP server on stdio.
func Serve(baseURL, token string) error {
	s := NewServer(baseURL, token)
	return server.ServeStdio(s)
}
