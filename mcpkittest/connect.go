// Package mcpkittest provides black-box MCP test connections.
package mcpkittest

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Connect creates an initialized in-memory client session and registers both
// sides for cleanup. It fails the current test on setup errors.
func Connect(tb testing.TB, server *mcp.Server) *mcp.ClientSession {
	tb.Helper()
	if server == nil {
		tb.Fatal("MCP server is required")
	}
	ctx := tb.Context()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		tb.Fatalf("connect MCP server: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpkit-test", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		tb.Fatalf("connect MCP client: %v", err)
	}
	tb.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Close()
	})
	return clientSession
}
