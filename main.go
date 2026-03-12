package main

import (
	"fmt"
	"os"

	"wezterm-mcp/config"
	"wezterm-mcp/wezterm"

	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "wezterm-mcp"
	serverVersion = "1.0.0"
)

func main() {
	// Load configuration (from --config flag or default config.yaml)
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	s := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	// Register all tools
	wezterm.RegisterTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
