package wezterm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

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

// BufferTracker tracks line counts per pane for read_new_lines.
type BufferTracker struct {
	paneLineCounts map[int]int
}

var bufferTracker = &BufferTracker{
	paneLineCounts: make(map[int]int),
}

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

var defaultPromptRE = regexp.MustCompile(`[$#%>]\s*$`)

// RegisterTools adds all WezTerm tools to the MCP server.
func RegisterTools(s *server.MCPServer) {
	registerListPanes(s)
	registerSendText(s)
	registerGetText(s)
	registerActivatePane(s)
	registerSendControlKey(s)
	registerRunCommandSync(s)
	registerReadNewLines(s)
}

// --- list_panes ---

func registerListPanes(s *server.MCPServer) {
	tool := mcp.NewTool("list_panes",
		mcp.WithDescription(
			"List all WezTerm terminal panes with their IDs, titles, working directories, and sizes. "+
				"Use this to discover which panes are available for interaction.",
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
				"Set newline=false to stage text without executing (user must press Enter).",
		),
		mcp.WithString("text", mcp.Required(), mcp.Description("Text or command to send to the terminal")),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently focused pane.")),
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

	if newline {
		text += "\n"
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	cliArgs := []string{"cli", "send-text", "--no-paste"}
	if paneID >= 0 {
		cliArgs = append(cliArgs, "--pane-id", strconv.Itoa(paneID))
	}

	_, stderr, err := runWeztermStdin(ctx, []byte(text), cliArgs...)
	if err != nil {
		return mcp.NewToolResultError(errorf("send_text", fmt.Sprintf("pane=%d", paneID), stderr, err)), nil
	}

	// Reset buffer tracker for this pane
	delete(bufferTracker.paneLineCounts, paneID)

	msg := fmt.Sprintf("Text sent to pane %d", paneID)
	if !newline {
		msg += " (no newline — user must press Enter to execute)"
	}
	return mcp.NewToolResultText(msg), nil
}

// --- get_text ---

func registerGetText(s *server.MCPServer) {
	tool := mcp.NewTool("get_text",
		mcp.WithDescription(
			"Read terminal output from a pane. Use lines=0 for visible screen only, "+
				"or specify a number to read from scrollback history.",
		),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently focused pane.")),
		mcp.WithNumber("lines", mcp.Description("Number of lines to read from scrollback (default: 50, 0=visible screen only)")),
	)
	s.AddTool(tool, getTextHandler)
}

func getTextHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	paneID := mcp.ParseInt(req, "pane_id", -1)
	lines := mcp.ParseInt(req, "lines", 50)

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
	if stdout == "" {
		return mcp.NewToolResultText("(empty output)"), nil
	}
	return mcp.NewToolResultText(stdout), nil
}

// --- activate_pane ---

func registerActivatePane(s *server.MCPServer) {
	tool := mcp.NewTool("activate_pane",
		mcp.WithDescription("Focus/activate a specific pane by its ID."),
		mcp.WithNumber("pane_id", mcp.Required(), mcp.Description("The pane ID to activate")),
	)
	s.AddTool(tool, activatePaneHandler)
}

func activatePaneHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	paneID := mcp.ParseInt(req, "pane_id", -1)
	if paneID < 0 {
		return mcp.NewToolResultError("pane_id is required"), nil
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	_, stderr, err := runWezterm(ctx, "cli", "activate-pane", "--pane-id", strconv.Itoa(paneID))
	if err != nil {
		return mcp.NewToolResultError(errorf("activate_pane", fmt.Sprintf("pane=%d", paneID), stderr, err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Activated pane %d", paneID)), nil
}

// --- send_control_key ---

func registerSendControlKey(s *server.MCPServer) {
	tool := mcp.NewTool("send_control_key",
		mcp.WithDescription(
			"Send a Ctrl+key sequence to a pane (e.g., 'c' for Ctrl+C to interrupt, 'd' for Ctrl+D for EOF).",
		),
		mcp.WithString("key", mcp.Required(), mcp.Description("Key to send with Ctrl (a, c, d, e, k, l, u, w, z)")),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently focused pane.")),
	)
	s.AddTool(tool, sendControlKeyHandler)
}

func sendControlKeyHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key := strings.ToLower(mcp.ParseString(req, "key", ""))
	paneID := mcp.ParseInt(req, "pane_id", -1)

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

// --- run_command_sync ---

func registerRunCommandSync(s *server.MCPServer) {
	tool := mcp.NewTool("run_command_sync",
		mcp.WithDescription(
			"Execute a command and wait for the shell prompt to return. "+
				"Returns the command output. Use for synchronous command execution. "+
				"Best suited for quick commands that complete quickly.",
		),
		mcp.WithString("command", mcp.Required(), mcp.Description("Command to execute")),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently focused pane.")),
		mcp.WithNumber("timeout_seconds", mcp.Description("Maximum wait time in seconds (default: 30)")),
		mcp.WithString("prompt_pattern", mcp.Description("Regex pattern to detect shell prompt (default: [$#%>]\\s*$)")),
	)
	s.AddTool(tool, runCommandSyncHandler)
}

func runCommandSyncHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command := mcp.ParseString(req, "command", "")
	if command == "" {
		return mcp.NewToolResultError("command parameter is required"), nil
	}
	paneID := mcp.ParseInt(req, "pane_id", -1)
	timeoutSec := mcp.ParseInt(req, "timeout_seconds", 30)
	promptPattern := mcp.ParseString(req, "prompt_pattern", "")

	promptRE := defaultPromptRE
	if promptPattern != "" {
		var err error
		promptRE, err = regexp.Compile(promptPattern)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid prompt_pattern: %v", err)), nil
		}
	}

	// Get baseline line count
	baseLines, err := fetchAllLines(ctx, paneID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read pane: %v", err)), nil
	}
	baseCount := len(baseLines)

	// Send command
	sendCtx, sendCancel := context.WithTimeout(ctx, defaultTimeout)
	cliArgs := []string{"cli", "send-text", "--no-paste"}
	if paneID >= 0 {
		cliArgs = append(cliArgs, "--pane-id", strconv.Itoa(paneID))
	}
	_, stderr, err := runWeztermStdin(sendCtx, []byte(command+"\n"), cliArgs...)
	sendCancel()
	if err != nil {
		return mcp.NewToolResultError(errorf("run_command_sync", "failed to send command", stderr, err)), nil
	}

	// Poll for prompt
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(300 * time.Millisecond)

		lines, err := fetchAllLines(ctx, paneID)
		if err != nil {
			continue
		}

		if len(lines) > baseCount {
			// Find last non-empty line
			lastNonEmpty := ""
			for i := len(lines) - 1; i >= 0; i-- {
				if strings.TrimSpace(lines[i]) != "" {
					lastNonEmpty = lines[i]
					break
				}
			}
			if promptRE.MatchString(lastNonEmpty) {
				newLines := lines[baseCount:]
				return mcp.NewToolResultText(strings.Join(newLines, "\n")), nil
			}
		}
	}

	// Timeout - return what we have
	lines, _ := fetchAllLines(ctx, paneID)
	if len(lines) > baseCount {
		return mcp.NewToolResultText(strings.Join(lines[baseCount:], "\n") + "\n(timeout — command may still be running)"), nil
	}
	return mcp.NewToolResultText("(timeout — no new output)"), nil
}

// --- read_new_lines ---

func registerReadNewLines(s *server.MCPServer) {
	tool := mcp.NewTool("read_new_lines",
		mcp.WithDescription(
			"Read only new lines that have appeared in the pane since the last call to this tool. "+
				"Use reset=true to initialize/reset the cursor without returning lines.",
		),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently focused pane.")),
		mcp.WithBoolean("reset", mcp.Description("Reset cursor to current buffer end without returning lines")),
	)
	s.AddTool(tool, readNewLinesHandler)
}

func readNewLinesHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	paneID := mcp.ParseInt(req, "pane_id", -1)
	reset := mcp.ParseBoolean(req, "reset", false)

	lines, err := fetchAllLines(ctx, paneID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read pane: %v", err)), nil
	}
	total := len(lines)

	if reset {
		bufferTracker.paneLineCounts[paneID] = total
		return mcp.NewToolResultText("(cursor reset)"), nil
	}

	last, known := bufferTracker.paneLineCounts[paneID]
	if !known {
		bufferTracker.paneLineCounts[paneID] = total
		return mcp.NewToolResultText("(cursor initialized — subsequent calls return new lines)"), nil
	}

	bufferTracker.paneLineCounts[paneID] = total

	if last >= total {
		return mcp.NewToolResultText("(no new lines)"), nil
	}

	newLines := lines[last:total]
	return mcp.NewToolResultText(strings.Join(newLines, "\n")), nil
}

// --- helpers ---

func fetchAllLines(ctx context.Context, paneID int) ([]string, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	cliArgs := []string{"cli", "get-text", "--start-line", "-99999"}
	if paneID >= 0 {
		cliArgs = append(cliArgs, "--pane-id", strconv.Itoa(paneID))
	}

	stdout, stderr, err := runWezterm(fetchCtx, cliArgs...)
	if err != nil {
		return nil, fmt.Errorf("%s", errorf("fetch_lines", fmt.Sprintf("pane=%d", paneID), stderr, err))
	}
	return strings.Split(stdout, "\n"), nil
}
