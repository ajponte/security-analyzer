package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"security-analyzer/pkg/llm"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPClient wraps the MCP SDK client and session.
type MCPClient struct {
	client  *mcp.Client
	session *mcp.ClientSession
}

// NewMCPClient spawns the server in a subprocess and connects to it.
func NewMCPClient(binaryPath string) (*MCPClient, error) {
	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "security-analyzer-cli-client",
			Version: "1.0.0",
		},
		nil,
	)

	// Command to start the server.
	cmd := exec.Command(binaryPath, "mcp")
	cmd.Stderr = os.Stderr // Redirect stderr for debug logs
	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	ctx := context.Background()
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	return &MCPClient{
		client:  client,
		session: session,
	}, nil
}

// ListTools queries the MCP server for available tools and converts them to llm.Tool definitions.
func (c *MCPClient) ListTools(ctx context.Context) ([]llm.Tool, error) {
	res, err := c.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from MCP server: %w", err)
	}

	tools := make([]llm.Tool, len(res.Tools))
	for i, t := range res.Tools {
		tools[i] = llm.Tool{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		}
	}
	return tools, nil
}

// CallTool executes a tool on the MCP server and returns its string result.
func (c *MCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (string, error) {
	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	}

	res, err := c.session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("tool call failed: %w", err)
	}

	if res.IsError {
		errMsg := "tool returned error"
		if len(res.Content) > 0 {
			if tc, ok := res.Content[0].(*mcp.TextContent); ok {
				errMsg = tc.Text
			}
		}
		return "", fmt.Errorf("tool execution error: %s", errMsg)
	}

	var resultText string
	for _, content := range res.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			resultText += tc.Text
		}
	}

	return resultText, nil
}

// Close terminates the MCP session.
func (c *MCPClient) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}
