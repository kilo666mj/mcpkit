package mcpkit_test

import (
	"context"
	"fmt"

	"github.com/kilo666mj/mcpkit"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listInput struct{}

type listOutput struct {
	Hosts []string `json:"hosts"`
}

func ExampleMustServer() {
	server := mcpkit.MustServer(mcpkit.ServerConfig{
		Name:         "inventory",
		Version:      "1.0.0",
		Instructions: "Use inventory tools for configured infrastructure facts.",
	})
	annotations := mcpkit.ReadOnly(false)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "inventory_list_hosts",
		Description: "List configured hosts.",
		Annotations: annotations,
	}, func(context.Context, *mcp.CallToolRequest, listInput) (*mcp.CallToolResult, listOutput, error) {
		return nil, listOutput{Hosts: []string{"web.example.com"}}, nil
	})

	fmt.Println(server != nil, annotations.ReadOnlyHint)
	// Output: true true
}
