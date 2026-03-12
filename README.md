# wezterm-mcp-go

MCP server for WezTerm terminal multiplexer. Enables LLM coding assistants to interact with terminal panes via the MCP protocol.

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
