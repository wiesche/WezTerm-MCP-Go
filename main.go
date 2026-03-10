package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"wezterm-mcp/wezterm"
)

const (
	serverName    = "wezterm-mcp"
	serverVersion = "1.0.0"
)

func main() {
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
