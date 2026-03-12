# WezTerm MCP Server

MCP server for WezTerm multiplexer access. Enables LLM coding assistants to interact with shared terminal panes via `wezterm cli` subprocess calls.

## Purpose

This server provides an alternative to direct CLI execution tools. It allows LLM assistants to:
- Discover and list available terminal panes
- Send commands and text to panes
- Read terminal output
- Execute synchronous commands that wait for completion
- Track new output incrementally

The user can review command executions and issue their own commands on shared terminal panes, enabling collaborative terminal sessions.

## Requirements

- [WezTerm](https://wezfurlong.org/wezterm/) installed and on system PATH
- Go 1.21+
- WezTerm mux server running (automatically started by WezTerm GUI)

## Build

```sh
go build -o wezterm-mcp .
# Windows
go build -o wezterm-mcp.exe .
```

## Configuration

The server looks for `config.yaml` in the same directory as the executable by default. You can also specify a custom path with `--config /path/to/config.yaml`.

### config.yaml

```yaml
# When true, prevents automatic command execution by filtering out
# carriage return (\r), newline (\n), and line feed characters from
# text sent to terminals. The user must manually press Enter to execute.
# Filtered characters are replaced with literal " \r " or " \n "
manual_command_execution: false
```

### Manual Command Execution Mode

When `manual_command_execution: true`:
- All `\r` and `\n` characters are filtered from `send_text` calls
- Filtered characters are replaced with ` \r ` or ` \n ` (space-padded)
- The response hints that manual mode is active and lists filtered characters
- `run_command_sync` is disabled (returns an error) since it requires execution

This mode is useful for collaborative sessions where the user wants to review and approve each command before execution.

## Tools

| Tool | Description |
|------|-------------|
| `list_panes` | List all panes with ID, title, working directory, and size |
| `send_text` | Send text/command to a pane; `newline=false` stages without executing |
| `get_text` | Read terminal output (`lines=0` for visible screen only) |
| `activate_pane` | Focus a pane by ID |
| `send_control_key` | Send Ctrl+key (a, c, d, e, k, l, u, w, z) |
| `run_command_sync` | Execute command, wait for shell prompt, return output (disabled in manual mode) |
| `read_new_lines` | Return lines added since last call (per-pane cursor tracking) |

All tools accept an optional `pane_id` parameter. If omitted, WezTerm targets the currently focused pane.

## MCP Client Configuration

### Windsurf / Cascade (`mcp_config.json`)

```json
{
  "mcpServers": {
    "wezterm": {
      "command": "C:/path/to/wezterm-mcp.exe",
      "args": []
    }
  }
}
```

With custom config:
```json
{
  "mcpServers": {
    "wezterm": {
      "command": "C:/path/to/wezterm-mcp.exe",
      "args": ["--config", "C:/path/to/config.yaml"]
    }
  }
}
```

### Claude Desktop (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "wezterm": {
      "command": "/path/to/wezterm-mcp",
      "args": []
    }
  }
}
```

## Usage Notes

- The server uses `--prefer-mux` to connect to the WezTerm multiplexer server
- Ensure WezTerm is running with the mux server enabled (default when GUI is running)
- For headless environments, start `wezterm-mux-server` manually
- `run_command_sync` detects shell prompts using regex `[$#%>]\s*$` by default
- `read_new_lines` maintains per-pane cursors to track what's been read
