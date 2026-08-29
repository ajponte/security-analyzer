package mcp

import (
	"context"
	"log/slog"
	"os"

	"security-analyzer/pkg/config"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server represents the MCP Server instance and its state.
type Server struct {
	mcpServer *mcp.Server
	cfg       *config.SemgrepConfig
	workspace string
}

// NewServer configures logging and instantiates the MCP server with an optional allowed workspace.
func NewServer(cfg *config.SemgrepConfig, workspace ...string) *Server {
	// Configure application logger to output to os.Stderr in MCP mode.
	// Since os.Stdout is used for MCP stdio transport, we must use os.Stderr for logs.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "security-analyzer-mcp",
			Version: "1.0.0",
		},
		nil,
	)

	// Resolve the allowed workspace directory.
	allowedWS := ""
	if len(workspace) > 0 && workspace[0] != "" {
		allowedWS = workspace[0]
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			slog.Error("failed to get current working directory for allowed workspace", "error", err)
			cwd = "."
		}
		allowedWS = cwd
	}

	return &Server{
		mcpServer: mcpServer,
		cfg:       cfg,
		workspace: allowedWS,
	}
}

// Start registers tools and runs the MCP server on stdio transport.
func (s *Server) Start(ctx context.Context) error {
	slog.Info("Starting MCP Server on stdio transport", "allowed_workspace", s.workspace)

	// Register Semgrep scan tool.
	registerSemgrepScanTool(s.mcpServer, s.cfg, s.workspace)

	// Run the server using StdioTransport.
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}
