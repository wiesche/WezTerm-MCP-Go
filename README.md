# wezterm-mcp-go

A simple MCP server for the [WezTerm](https://wezterm.org/) terminal multiplexer.
Use this for CLI command execution in any MCP capable LLM assistant or IDE (Claude Desktop or Code, Cursor, Windsurf, Codename Goose, OpenCode, etc.). It will enable you and your assistant to edit and execute terminal commands in a shared terminal session within the fabulous WezTerm terminal for any shell of your liking (PowerShell, cmd.exe, Bash, Zsh, etc.) in the same chat. WezTerm as well as this MCP server run natively on both, Linux (incl. macOS) and Windows systems and pose a way more powerful alternative to TMUX based collaborative terminal use.

Use WezTerm via wezterm-mcp-go in order to:

1. Make your terminal available to any LLM assistant 🖥️
2. Add context to your chat session using CLI commands 💬
3. Have your assistant execute commands in your terminal directly ⚡
4. Have your assistant suggest a command for you to review and edit before execution by activating `manual_command_execution` mode in the config.yaml ✏️
5. Use any shell installed on your system as a pane within WezTerm without needing to register it in the config, before 🐚
6. Use any app which has a terminal user interface (TUI) collaboratively with your assistant in a connected WezTerm pane 🖥️


The initial version of this project was created after [hiraishikentaro's TS implementation](https://github.com/hiraishikentaro/wezterm-mcp/) as reference.

## Requirements

- [WezTerm](https://wezfurlong.org/wezterm/) on PATH
- WezTerm mux server running (usually activated on spawning panes via MCP)
- Go 1.21+ (just for compiling)

## Installation

```sh
git clone https://github.com/yourusername/wezterm-mcp-go.git
cd wezterm-mcp-go
go build -o wezterm-mcp-go .
```
Add the wezterm-mcp-go executable as a simple STDIO MCP server to the config of your favorite AI buddy. No other parameters needed. A config.yaml in the same directory or as parameter `--config /path/to/config.yaml` is optional.

## Configuration

Optional `config.yaml` (same directory as executable, or `--config /path/to/config.yaml`):

```yaml
# Prevent automatic command execution - user must press Enter manually
manual_command_execution: false
```
(more to come)

## Tools

| Tool | Description |
|------|-------------|
| `list_panes` | List all terminal panes |
| `spawn_new_shell` | Spawn a new pane with a shell (starts WezTerm GUI if needed) |
| `get_text` | Read terminal output |
| `send_text` | Send text/command to pane |
| `send_control_key` | Send Ctrl+key (C, D, etc.) |

### Pane Management

Panes need to be part of a unix domain (workspace) in order to be reachable over the WezTerm mux server and be visible to the MCP server (works on Windows the same way). New panes started using the MCP server will be part of domain 'local' and be visible by default. You can start new panes running any shell installed on your system or even available as portable executable binary. Just give your AI assistant the path to the shell executable.

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
