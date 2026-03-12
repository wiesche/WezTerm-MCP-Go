package wezterm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"wezterm-mcp/config"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// PaneInfo mirrors JSON from `wezterm cli list --format json`.
type PaneInfo struct {
	WindowID  int    `json:"window_id"`
	TabID     int    `json:"tab_id"`
	PaneID    int    `json:"pane_id"`
	Workspace string `json:"workspace"`
	Size      struct {
		Rows int `json:"rows"`
		Cols int `json:"cols"`
	} `json:"size"`
	Title string `json:"title"`
	CWD   string `json:"cwd"`
}

// ActivePane tracks the currently active pane ID.
// When a tool is called with an explicit pane_id, that pane becomes active.
var activePaneID = -1

// ManualModeState tracks state for manual mode hinting
type ManualModeState struct {
	sendTextCallCount int
	firstCallDone     bool
}

var manualModeState = &ManualModeState{}

// controlBytes maps key names to Ctrl+key byte sequences.
var controlBytes = map[string][]byte{
	"a": {0x01},
	"c": {0x03},
	"d": {0x04},
	"e": {0x05},
	"k": {0x0b},
	"l": {0x0c},
	"u": {0x15},
	"w": {0x17},
	"z": {0x1a},
}

// filterExecutionChars removes carriage return, newline, and line feed characters
// when manual_command_execution is enabled. Returns the filtered text and a list
// of filtered character descriptions for reporting.
func filterExecutionChars(text string) (string, []string) {
	if config.Active == nil || !config.Active.ManualCommandExecution {
		return text, nil
	}

	var filtered []string
	var result strings.Builder
	runes := []rune(text)

	for i, r := range runes {
		switch r {
		case '\r':
			// Replace with literal " \r " unless at end of line
			if i+1 < len(runes) && runes[i+1] != '\n' {
				result.WriteString(" \\r ")
				filtered = append(filtered, "\\r (carriage return)")
			} else {
				result.WriteString(" \\r ")
				filtered = append(filtered, "\\r (carriage return)")
			}
		case '\n':
			// Replace with literal " \n " unless at end of text
			if i+1 < len(runes) {
				result.WriteString(" \\n ")
				filtered = append(filtered, "\\n (newline)")
			} else {
				result.WriteString(" \\n ")
				filtered = append(filtered, "\\n (newline)")
			}
		default:
			result.WriteRune(r)
		}
	}

	return result.String(), filtered
}

// RegisterTools adds all WezTerm tools to the MCP server.
func RegisterTools(s *server.MCPServer) {
	registerListPanes(s)
	registerSendText(s)
	registerGetText(s)
	registerSendControlKey(s)
}

// --- list_panes ---

func registerListPanes(s *server.MCPServer) {
	tool := mcp.NewTool("list_panes",
		mcp.WithDescription(
			"List all WezTerm terminal panes with their IDs, titles, working directories, and sizes. "+
				"Use this to discover which panes are available for interaction. "+
				"Use get_text to determine which pane is currently active.",
		),
	)
	s.AddTool(tool, listPanesHandler)
}

func listPanesHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	stdout, stderr, err := runWezterm(ctx, "cli", "list", "--format", "json")
	if err != nil {
		return mcp.NewToolResultError(errorf("list_panes", "failed to list panes", stderr, err)), nil
	}

	var panes []PaneInfo
	if err := json.Unmarshal([]byte(stdout), &panes); err != nil {
		return mcp.NewToolResultText(stdout), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-8s %-8s %-8s %-12s %-10s %s\n", "PANEID", "WINDOW", "TAB", "WORKSPACE", "SIZE", "TITLE"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	for _, p := range panes {
		size := fmt.Sprintf("%dx%d", p.Size.Cols, p.Size.Rows)
		sb.WriteString(fmt.Sprintf("%-8d %-8d %-8d %-12s %-10s %s\n",
			p.PaneID, p.WindowID, p.TabID, p.Workspace, size, p.Title))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

// --- send_text ---

func registerSendText(s *server.MCPServer) {
	tool := mcp.NewTool("send_text",
		mcp.WithDescription(
			"Send text or a command to a WezTerm pane. By default appends a newline to execute immediately. "+
				"Set newline=false to stage text without executing (user must press Enter). "+
				"When pane_id is specified, that pane becomes the active pane. "+
				"Use get_text to determine which pane is currently active.",
		),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text or command to send to the terminal")),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently active pane. If specified, this pane becomes active.")),
		mcp.WithBoolean("newline", mcp.Description("Append newline to execute immediately (default: true)")),
	)
	s.AddTool(tool, sendTextHandler)
}

func sendTextHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	text := mcp.ParseString(req, "text", "")
	if text == "" {
		return mcp.NewToolResultError("text parameter is required"), nil
	}
	paneID := mcp.ParseInt(req, "pane_id", -1)
	newline := mcp.ParseBoolean(req, "newline", true)

	// If pane_id is specified, it becomes the active pane
	if paneID >= 0 {
		activePaneID = paneID
	}

	// Apply newline if requested (before filtering, so it gets filtered in manual mode)
	if newline {
		text += "\n"
	}

	// Filter execution characters if manual_command_execution is enabled
	filteredText, filteredChars := filterExecutionChars(text)

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	cliArgs := []string{"cli", "send-text", "--no-paste"}
	if paneID >= 0 {
		cliArgs = append(cliArgs, "--pane-id", strconv.Itoa(paneID))
	}

	_, stderr, err := runWeztermStdin(ctx, []byte(filteredText), cliArgs...)
	if err != nil {
		return mcp.NewToolResultError(errorf("send_text", fmt.Sprintf("pane=%d", paneID), stderr, err)), nil
	}

	// Build response message
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("Text sent to pane %d", paneID))

	// Determine if we should show the manual mode hint
	if config.Active != nil && config.Active.ManualCommandExecution {
		manualModeState.sendTextCallCount++
		showHint := !manualModeState.firstCallDone ||
			len(filteredChars) > 0 ||
			manualModeState.sendTextCallCount%10 == 0

		if showHint {
			manualModeState.firstCallDone = true
			msg.WriteString("\n\n[MANUAL MODE] Configuration set for user to execute sent commands manually.")
			msg.WriteString(" Set 'manual_command_execution: false' in config.yaml to disable.")
			if len(filteredChars) > 0 {
				msg.WriteString("\nFiltered characters: ")
				msg.WriteString(strings.Join(filteredChars, ", "))
			}
		}
	} else if !newline {
		msg.WriteString(" (no newline — user must press Enter to execute)")
	}

	return mcp.NewToolResultText(msg.String()), nil
}

// --- get_text ---

func registerGetText(s *server.MCPServer) {
	tool := mcp.NewTool("get_text",
		mcp.WithDescription(
			"Read terminal output from a pane. Use lines=0 for visible screen only, "+
				"or specify a number to read from scrollback history. "+
				"When pane_id is specified, that pane becomes the active pane. "+
				"The response includes the pane_id to help determine which pane is active.",
		),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently active pane. If specified, this pane becomes active.")),
		mcp.WithNumber("lines", mcp.Description("Number of lines to read from scrollback (default: 50, 0=visible screen only)")),
	)
	s.AddTool(tool, getTextHandler)
}

func getTextHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	paneID := mcp.ParseInt(req, "pane_id", -1)
	lines := mcp.ParseInt(req, "lines", 50)

	// If pane_id is specified, it becomes the active pane
	if paneID >= 0 {
		activePaneID = paneID
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	cliArgs := []string{"cli", "get-text"}
	if paneID >= 0 {
		cliArgs = append(cliArgs, "--pane-id", strconv.Itoa(paneID))
	}
	if lines > 0 {
		cliArgs = append(cliArgs, "--start-line", strconv.Itoa(-lines))
	}

	stdout, stderr, err := runWezterm(ctx, cliArgs...)
	if err != nil {
		return mcp.NewToolResultError(errorf("get_text", fmt.Sprintf("pane=%d lines=%d", paneID, lines), stderr, err)), nil
	}

	// Build response with pane_id info
	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("[PANE %d]\n", paneID))
	if stdout == "" {
		msg.WriteString("(empty output)")
	} else {
		msg.WriteString(stdout)
	}
	return mcp.NewToolResultText(msg.String()), nil
}

// --- send_control_key ---

func registerSendControlKey(s *server.MCPServer) {
	tool := mcp.NewTool("send_control_key",
		mcp.WithDescription(
			"Send a Ctrl+key sequence to a pane (e.g., 'c' for Ctrl+C to interrupt, 'd' for Ctrl+D for EOF). "+
				"When pane_id is specified, that pane becomes the active pane. "+
				"Use get_text to determine which pane is currently active.",
		),
		mcp.WithString("key", mcp.Required(), mcp.Description("Key to send with Ctrl (a, c, d, e, k, l, u, w, z)")),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently active pane. If specified, this pane becomes active.")),
	)
	s.AddTool(tool, sendControlKeyHandler)
}

func sendControlKeyHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key := strings.ToLower(mcp.ParseString(req, "key", ""))
	paneID := mcp.ParseInt(req, "pane_id", -1)

	// If pane_id is specified, it becomes the active pane
	if paneID >= 0 {
		activePaneID = paneID
	}

	seq, ok := controlBytes[key]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown control key '%s'; valid keys: a, c, d, e, k, l, u, w, z", key)), nil
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	cliArgs := []string{"cli", "send-text", "--no-paste"}
	if paneID >= 0 {
		cliArgs = append(cliArgs, "--pane-id", strconv.Itoa(paneID))
	}

	_, stderr, err := runWeztermStdin(ctx, seq, cliArgs...)
	if err != nil {
		return mcp.NewToolResultError(errorf("send_control_key", fmt.Sprintf("key=Ctrl+%s pane=%d", strings.ToUpper(key), paneID), stderr, err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Sent Ctrl+%s to pane %d", strings.ToUpper(key), paneID)), nil
}
