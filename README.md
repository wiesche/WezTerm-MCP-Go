# wezterm-mcp-go

A simple STDIO based MCP server for the [WezTerm](https://wezterm.org/) terminal multiplexer. Using this server for CLI command execution in LLM assistants like Claude Code allows for collaborative command line prompt use in the fabulous WezTerm terminal for any shell (PowerShell, CMD, WSL Bash, etc.) WezTerm as well as this MCP server natively run on both, Linux and Windows systems and is a more powerful alternative for TMUX based collaborative terminal use.

Use WezTerm via wezterm-mcp-go in order to:

1. Make your terminal available to an LLM assistant 🖥️
2. Add context to your chat session using CLI commands 💬
3. Have your assistant execute commands in your terminal directly ⚡
4. Have your assistant suggest a command for you to review and edit before execution by activating `manual_command_execution` mode in the config.yaml ✏️
5. Use any shell installed on your system as a pane within WezTerm without needing to register it in the config, before 🐚
6. Use any app which has a terminal user interface (TUI) collaboratively with your assistant in a connected WezTerm pane 🖥️

, natively on either Windows, Linux, or macOS systems.

The initial version of this project was created with [hiraishikentaro's TS implementation](https://github.com/hiraishikentaro/wezterm-mcp/) for reference.

## Features

- List and discover terminal panes
- Send commands and text to panes
- Read terminal output (scrollback/visible)
- Send control sequences (Ctrl+C, Ctrl+D, etc.)
- Optional manual execution mode (review commands before execution)

## Requirements

- [WezTerm](https://wezfurlong.org/wezterm/) on PATH
- Go 1.21+
- WezTerm mux server running (automatic with GUI)

## Installation

```sh
git clone https://github.com/yourusername/wezterm-mcp-go.git
cd wezterm-mcp-go
go build -o wezterm-mcp-go .
```

## Configuration

Optional `config.yaml` (same directory as executable, or `--config /path/to/config.yaml`):

```yaml
# Prevent automatic command execution - user must press Enter manually
manual_command_execution: false
```

## Tools

| Tool | Description |
|------|-------------|
| `list_panes` | List all terminal panes |
| `get_text` | Read terminal output |
| `send_text` | Send text/command to pane |
| `send_control_key` | Send Ctrl+key (C, D, etc.) |

### Pane Management

- Auto-selects lowest pane ID when no pane specified
- Specifying `pane_id` makes it active for subsequent calls
- Returns available panes when requested pane doesn't exist

### Response Format

```json
{
  "pane_id": 2,
  "auto_selected": true,
  "output": "terminal content..."
}
```

## MCP Client Setup

**Windsurf / Cascade:**
```json
{
  "mcpServers": {
    "wezterm": {
      "command": "/path/to/wezterm-mcp-go"
    }
  }
}
```

**Claude Desktop:**
```json
{
  "mcpServers": {
    "wezterm": {
      "command": "/path/to/wezterm-mcp-go"
    }
  }
}
```

## License

MIT
