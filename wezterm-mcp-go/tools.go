package wezterm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"wezterm-mcp-go/config"

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
// -1 means undefined - will auto-select lowest pane ID on first use.
// Only set when explicitly provided in a tool call.
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

	for _, r := range runes {
		switch r {
		case '\r':
			result.WriteString(" \\r ")
			filtered = append(filtered, "\\r (carriage return)")
		case '\n':
			result.WriteString(" \\n ")
			filtered = append(filtered, "\\n (newline)")
		default:
			result.WriteRune(r)
		}
	}

	return result.String(), filtered
}

// fetchPaneList retrieves the list of available panes.
func fetchPaneList(ctx context.Context) ([]PaneInfo, error) {
	stdout, stderr, err := runWezterm(ctx, "cli", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("%s", errorf("list_panes", "failed to list panes", stderr, err))
	}

	var panes []PaneInfo
	if err := json.Unmarshal([]byte(stdout), &panes); err != nil {
		return nil, fmt.Errorf("failed to parse pane list: %w", err)
	}
	return panes, nil
}

// resolvePaneID determines the effective pane ID to use.
// If explicitID >= 0, it's used and becomes the active pane.
// If activePaneID >= 0, it's used (but not set as active).
// Otherwise, auto-selects the lowest available pane ID.
// Returns the pane ID, whether it was auto-selected, and any error.
func resolvePaneID(ctx context.Context, explicitID int) (paneID int, autoSelected bool, err error) {
	// If explicit pane_id provided, use it and make it active
	if explicitID >= 0 {
		return explicitID, false, nil
	}

	// If we have an active pane, use it
	if activePaneID >= 0 {
		return activePaneID, false, nil
	}

	// No active pane - auto-select lowest pane ID
	panes, err := fetchPaneList(ctx)
	if err != nil {
		return -1, false, err
	}
	if len(panes) == 0 {
		return -1, false, fmt.Errorf("no panes available")
	}

	// Find lowest pane ID
	lowestID := panes[0].PaneID
	for _, p := range panes[1:] {
		if p.PaneID < lowestID {
			lowestID = p.PaneID
		}
	}
	return lowestID, true, nil
}

// validatePaneExists checks if a pane ID exists in the current pane list.
func validatePaneExists(ctx context.Context, paneID int) (bool, []PaneInfo, error) {
	panes, err := fetchPaneList(ctx)
	if err != nil {
		return false, nil, err
	}
	for _, p := range panes {
		if p.PaneID == paneID {
			return true, panes, nil
		}
	}
	return false, panes, nil
}

// formatPaneList returns a formatted string of available panes.
func formatPaneList(panes []PaneInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-8s %-8s %-8s %-12s %-10s %s\n", "PANEID", "WINDOW", "TAB", "WORKSPACE", "SIZE", "TITLE"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")
	for _, p := range panes {
		size := fmt.Sprintf("%dx%d", p.Size.Cols, p.Size.Rows)
		sb.WriteString(fmt.Sprintf("%-8d %-8d %-8d %-12s %-10s %s\n",
			p.PaneID, p.WindowID, p.TabID, p.Workspace, size, p.Title))
	}
	return sb.String()
}

// RegisterTools adds all WezTerm tools to the MCP server.
func RegisterTools(s *server.MCPServer) {
	registerListPanes(s)
	registerSendText(s)
	registerGetText(s)
	registerSendControlKey(s)
	registerSpawnNewShell(s)
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

	// Check for GUI warning
	var warnings []string
	if !IsWezTermGUIRunning() {
		warnings = append(warnings, "No WezTerm GUI instance found on system. Start WezTerm to enable pane operations.")
	}

	panes, err := fetchPaneList(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"panes": formatPaneList(panes),
	}
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// --- send_text ---

func registerSendText(s *server.MCPServer) {
	tool := mcp.NewTool("send_text",
		mcp.WithDescription(
			"Send text or a command to a WezTerm pane. By default appends a newline to execute immediately. "+
				"Set newline=false to stage text without executing (user must press Enter). "+
				"If pane_id is specified, that pane becomes the active pane for subsequent calls.",
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
	explicitPaneID := mcp.ParseInt(req, "pane_id", -1)
	newline := mcp.ParseBoolean(req, "newline", true)

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Check for GUI warning
	var warnings []string
	if !IsWezTermGUIRunning() {
		warnings = append(warnings, "No WezTerm GUI instance found on system. Start WezTerm to enable pane operations.")
	}

	// Resolve pane ID
	paneID, autoSelected, err := resolvePaneID(ctx, explicitPaneID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to determine pane: %v", err)), nil
	}

	// Validate pane exists
	exists, panes, err := validatePaneExists(ctx, paneID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to validate pane: %v", err)), nil
	}
	if !exists {
		// Pane no longer exists - only reset active pane if this was the active pane
		wasActive := paneID == activePaneID
		if wasActive {
			activePaneID = -1
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Pane %d is no longer available.", paneID))
		if wasActive {
			sb.WriteString(" Active pane reset.")
		}
		sb.WriteString("\n\nAvailable panes:\n")
		sb.WriteString(formatPaneList(panes))
		return mcp.NewToolResultError(sb.String()), nil
	}

	// If explicit pane_id provided, make it active
	if explicitPaneID >= 0 {
		activePaneID = explicitPaneID
	}

	// Apply newline if requested (before filtering, so it gets filtered in manual mode)
	if newline {
		text += "\n"
	}

	// Filter execution characters if manual_command_execution is enabled
	filteredText, filteredChars := filterExecutionChars(text)

	cliArgs := []string{"cli", "send-text", "--no-paste"}
	cliArgs = append(cliArgs, "--pane-id", strconv.Itoa(paneID))

	_, stderr, err := runWeztermStdin(ctx, []byte(filteredText), cliArgs...)
	if err != nil {
		return mcp.NewToolResultError(errorf("send_text", fmt.Sprintf("pane=%d", paneID), stderr, err)), nil
	}

	// Build response with pane_id as structured data
	result := map[string]interface{}{
		"pane_id":       paneID,
		"auto_selected": autoSelected,
		"message":       fmt.Sprintf("Text sent to pane %d", paneID),
	}
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}

	// Determine if we should show the manual mode hint
	if config.Active != nil && config.Active.ManualCommandExecution {
		manualModeState.sendTextCallCount++
		showHint := !manualModeState.firstCallDone ||
			len(filteredChars) > 0 ||
			manualModeState.sendTextCallCount%10 == 0

		if showHint {
			manualModeState.firstCallDone = true
			result["manual_mode_hint"] = "Configuration set for user to execute sent commands manually. Set 'manual_command_execution: false' in config.yaml to disable."
			if len(filteredChars) > 0 {
				result["filtered_characters"] = filteredChars
			}
		}
	} else if !newline {
		result["message"] = fmt.Sprintf("Text sent to pane %d (no newline — user must press Enter to execute)", paneID)
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// --- get_text ---

func registerGetText(s *server.MCPServer) {
	tool := mcp.NewTool("get_text",
		mcp.WithDescription(
			"Read terminal output from a pane. Use lines=0 for visible screen only, "+
				"or specify a number to read from scrollback history. "+
				"If pane_id is specified, that pane becomes the active pane for subsequent calls. "+
				"Returns pane_id in the response to help identify the active pane.",
		),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently active pane. If specified, this pane becomes active.")),
		mcp.WithNumber("lines", mcp.Description("Number of lines to read from scrollback (default: 50, 0=visible screen only)")),
	)
	s.AddTool(tool, getTextHandler)
}

func getTextHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	explicitPaneID := mcp.ParseInt(req, "pane_id", -1)
	lines := mcp.ParseInt(req, "lines", 50)

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Check for GUI warning
	var warnings []string
	if !IsWezTermGUIRunning() {
		warnings = append(warnings, "No WezTerm GUI instance found on system. Start WezTerm to enable pane operations.")
	}

	// Resolve pane ID
	paneID, autoSelected, err := resolvePaneID(ctx, explicitPaneID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to determine pane: %v", err)), nil
	}

	// Validate pane exists
	exists, panes, err := validatePaneExists(ctx, paneID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to validate pane: %v", err)), nil
	}
	if !exists {
		// Pane no longer exists - only reset active pane if this was the active pane
		wasActive := paneID == activePaneID
		if wasActive {
			activePaneID = -1
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Pane %d is no longer available.", paneID))
		if wasActive {
			sb.WriteString(" Active pane reset.")
		}
		sb.WriteString("\n\nAvailable panes:\n")
		sb.WriteString(formatPaneList(panes))
		return mcp.NewToolResultError(sb.String()), nil
	}

	// If explicit pane_id provided, make it active
	if explicitPaneID >= 0 {
		activePaneID = explicitPaneID
	}

	cliArgs := []string{"cli", "get-text"}
	cliArgs = append(cliArgs, "--pane-id", strconv.Itoa(paneID))
	if lines > 0 {
		cliArgs = append(cliArgs, "--start-line", strconv.Itoa(-lines))
	}

	stdout, stderr, err := runWezterm(ctx, cliArgs...)
	if err != nil {
		return mcp.NewToolResultError(errorf("get_text", fmt.Sprintf("pane=%d lines=%d", paneID, lines), stderr, err)), nil
	}

	// Add execution hint if manual mode is not active
	if config.Active == nil || !config.Active.ManualCommandExecution {
		warnings = append(warnings, "Commands must end with a newline (Enter) to execute. Use send_text with newline=true, or send_control_key with key='m' (Ctrl+M) to send a carriage return.")
	}

	// Build response with pane_id as structured data
	result := map[string]interface{}{
		"pane_id":       paneID,
		"auto_selected": autoSelected,
		"output":        stdout,
	}
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// --- send_control_key ---

func registerSendControlKey(s *server.MCPServer) {
	tool := mcp.NewTool("send_control_key",
		mcp.WithDescription(
			"Send a Ctrl+key sequence to a pane (e.g., 'c' for Ctrl+C to interrupt, 'd' for Ctrl+D for EOF). "+
				"If pane_id is specified, that pane becomes the active pane for subsequent calls.",
		),
		mcp.WithString("key", mcp.Required(), mcp.Description("Key to send with Ctrl (a, c, d, e, k, l, u, w, z)")),
		mcp.WithNumber("pane_id", mcp.Description("Target pane ID. Omit to use the currently active pane. If specified, this pane becomes active.")),
	)
	s.AddTool(tool, sendControlKeyHandler)
}

func sendControlKeyHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key := strings.ToLower(mcp.ParseString(req, "key", ""))
	explicitPaneID := mcp.ParseInt(req, "pane_id", -1)

	seq, ok := controlBytes[key]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("unknown control key '%s'; valid keys: a, c, d, e, k, l, u, w, z", key)), nil
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Check for GUI warning
	var warnings []string
	if !IsWezTermGUIRunning() {
		warnings = append(warnings, "No WezTerm GUI instance found on system. Start WezTerm to enable pane interaction.")
	}

	// Resolve pane ID
	paneID, autoSelected, err := resolvePaneID(ctx, explicitPaneID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to determine pane: %v", err)), nil
	}

	// Validate pane exists
	exists, panes, err := validatePaneExists(ctx, paneID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to validate pane: %v", err)), nil
	}
	if !exists {
		// Pane no longer exists - only reset active pane if this was the active pane
		wasActive := paneID == activePaneID
		if wasActive {
			activePaneID = -1
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Pane %d is no longer available.", paneID))
		if wasActive {
			sb.WriteString(" Active pane reset.")
		}
		sb.WriteString("\n\nAvailable panes:\n")
		sb.WriteString(formatPaneList(panes))
		return mcp.NewToolResultError(sb.String()), nil
	}

	// If explicit pane_id provided, make it active
	if explicitPaneID >= 0 {
		activePaneID = explicitPaneID
	}

	cliArgs := []string{"cli", "send-text", "--no-paste"}
	cliArgs = append(cliArgs, "--pane-id", strconv.Itoa(paneID))

	_, stderr, err := runWeztermStdin(ctx, seq, cliArgs...)
	if err != nil {
		return mcp.NewToolResultError(errorf("send_control_key", fmt.Sprintf("key=Ctrl+%s pane=%d", strings.ToUpper(key), paneID), stderr, err)), nil
	}

	// Build response with pane_id as structured data
	result := map[string]interface{}{
		"pane_id":       paneID,
		"auto_selected": autoSelected,
		"message":       fmt.Sprintf("Sent Ctrl+%s to pane %d", strings.ToUpper(key), paneID),
	}
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// --- spawn_new_shell ---

func registerSpawnNewShell(s *server.MCPServer) {
	tool := mcp.NewTool("spawn_new_shell",
		mcp.WithDescription(
			"Spawn a new terminal pane with a shell. Starts WezTerm GUI if not running. "+
				"The new pane becomes the active pane. Returns the new pane ID.",
		),
		mcp.WithString("shell", mcp.Description("Shell command or path to start (e.g., 'powershell', 'bash', 'wsl'). Default: system default shell.")),
		mcp.WithString("domain", mcp.Description("Domain name for spawn (default: 'local'). Use 'ssh:hostname' for remote domains.")),
	)
	s.AddTool(tool, spawnNewShellHandler)
}

func spawnNewShellHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	shell := mcp.ParseString(req, "shell", "")
	domain := mcp.ParseString(req, "domain", "local")

	// Check if wezterm is on PATH
	if !CheckWezTermOnPath() {
		return mcp.NewToolResultError(
			"WezTerm is not installed or not on PATH.\n" +
				"Get WezTerm for free at: https://wezterm.org/"), nil
	}

	// Check if WezTerm GUI is running, start if not
	guiWasStarted := false
	if !IsWezTermGUIRunning() {
		if err := StartWezTermGUI(ctx); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to start WezTerm GUI: %v", err)), nil
		}
		guiWasStarted = true
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	// Build spawn command: wezterm cli spawn --domain-name <domain> -- <shell>
	cliArgs := []string{"cli", "spawn", "--domain-name", domain}
	if shell != "" {
		cliArgs = append(cliArgs, "--", shell)
	}

	stdout, stderr, err := runWezterm(ctx, cliArgs...)
	if err != nil {
		// Check if this is the "pane not specified" error indicating GUI wasn't ready
		if strings.Contains(stderr, "--pane-id was not specified") ||
			strings.Contains(stderr, "$WEZTERM_PANE") {
			result := map[string]interface{}{
				"error":   errorf("spawn_new_shell", fmt.Sprintf("domain=%s shell=%s", domain, shell), stderr, err),
				"warning": "No pane added. The WezTerm GUI process needed to spin up first in the last tool call. Repeat in order to add a new pane.",
			}
			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			return mcp.NewToolResultText(string(resultJSON)), nil
		}
		return mcp.NewToolResultError(errorf("spawn_new_shell", fmt.Sprintf("domain=%s shell=%s", domain, shell), stderr, err)), nil
	}

	// Parse the output to get the new pane ID
	// Output format: "pane_id:7" or just the number
	newPaneIDStr := strings.TrimSpace(stdout)
	newPaneIDStr = strings.TrimPrefix(newPaneIDStr, "pane_id:")
	newPaneID, err := strconv.Atoi(newPaneIDStr)
	if err != nil {
		// Try to extract number from output
		for _, part := range strings.Fields(stdout) {
			if id, e := strconv.Atoi(part); e == nil {
				newPaneID = id
				break
			}
		}
		if newPaneID == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to parse pane ID from spawn output: %s", stdout)), nil
		}
	}

	// Activate the new pane
	_, stderr, err = runWezterm(ctx, "cli", "activate-pane", "--pane-id", strconv.Itoa(newPaneID))
	if err != nil {
		// Non-fatal: pane was created but activation failed
		// Continue and return success with warning
	}

	// Set as active pane
	activePaneID = newPaneID

	// Build response
	result := map[string]interface{}{
		"pane_id":     newPaneID,
		"domain":      domain,
		"shell":       shell,
		"gui_started": guiWasStarted,
		"message":     fmt.Sprintf("Spawned new pane %d", newPaneID),
	}
	if shell == "" {
		result["shell"] = "(default)"
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
