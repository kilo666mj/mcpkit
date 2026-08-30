package mcpkittest

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConnect(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	type output struct {
		Value string `json:"value"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "read"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, output, error) {
		return nil, output{Value: "ok"}, nil
	})
	session := Connect(t, server)
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "read"})
	if err != nil || result.IsError {
		t.Fatalf("CallTool() result=%+v error=%v", result, err)
	}
}
