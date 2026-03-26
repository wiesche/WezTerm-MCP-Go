package paneops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"wezterm-mcp-go/config"
)

const defaultTimeout = 10 * time.Second

// Platform holds OS-specific binary names.
type Platform struct {
	WezTermBin     string
	GUIProcessName string
}

// DetectPlatform returns OS-specific configuration.
func DetectPlatform() Platform {
	if runtime.GOOS == "windows" {
		return Platform{
			WezTermBin:     "wezterm.exe",
			GUIProcessName: "wezterm-gui.exe",
		}
	}
	return Platform{
		WezTermBin:     "wezterm",
		GUIProcessName: "wezterm",
	}
}

var platform = DetectPlatform()

// DetectShellType identifies the shell type from pane title for profile lookup.
func DetectShellType(paneTitle string) string {
	title := strings.ToLower(paneTitle)

	// Windows shells
	if strings.Contains(title, "powershell") || strings.Contains(title, "pwsh") {
		return "powershell"
	}
	if strings.Contains(title, "cmd.exe") || (strings.Contains(title, "cmd") && strings.Contains(title, ".exe")) {
		return "cmd"
	}

	// Linux/WSL shells
	if strings.Contains(title, "wsl") || strings.Contains(title, "ubuntu") || strings.Contains(title, "debian") {
		return "wsl"
	}
	if strings.Contains(title, "bash") {
		return "bash"
	}
	if strings.Contains(title, "zsh") {
		return "bash" // zsh uses same sequences as bash
	}
	if strings.Contains(title, "fish") {
		return "bash" // fish uses same sequences as bash for clear/enter
	}

	return "unknown"
}

// GetShellProfile returns the shell profile for a given shell type from config.
func GetShellProfile(shellType string) config.ShellProfile {
	if config.Active != nil {
		if profile, ok := config.Active.ShellProfiles[shellType]; ok {
			return profile
		}
	}
	// Default: enter=\n, clear_line=""
	return config.ShellProfile{Enter: "\n", ClearLine: ""}
}

// runWezterm executes "wezterm cli --prefer-mux <args>" and returns stdout.
func runWezterm(ctx context.Context, args ...string) (string, string, error) {
	var full []string
	if len(args) > 0 && args[0] == "cli" {
		full = append([]string{"cli", "--prefer-mux"}, args[1:]...)
	} else {
		full = args
	}
	cmd := exec.CommandContext(ctx, platform.WezTermBin, full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// runWeztermStdin executes wezterm with data piped to stdin.
func runWeztermStdin(ctx context.Context, stdin []byte, args ...string) (string, string, error) {
	var full []string
	if len(args) > 0 && args[0] == "cli" {
		full = append([]string{"cli", "--prefer-mux"}, args[1:]...)
	} else {
		full = args
	}
	cmd := exec.CommandContext(ctx, platform.WezTermBin, full...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// SendTextToPane sends text to a pane (without newline).
func SendTextToPane(paneID int, text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cliArgs := []string{"cli", "send-text", "--no-paste", "--pane-id", strconv.Itoa(paneID)}
	_, _, err := runWeztermStdin(ctx, []byte(text), cliArgs...)
	return err
}

// SendTextWithNewline sends text to a pane with appropriate newline for shell type.
// Sends text via --no-paste, then End key, then Enter - all as separate calls.
func SendTextWithNewline(paneID int, text string, shellType string) error {
	profile := GetShellProfile(shellType)
	paneArg := strconv.Itoa(paneID)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// 1. Send the text without newline (--no-paste to send raw)
	if _, _, err := runWeztermStdin(ctx, []byte(text),
		"cli", "send-text", "--no-paste", "--pane-id", paneArg); err != nil {
		return err
	}

	// 2. Send End key to move cursor to end of line
	if _, _, err := runWeztermStdin(ctx, []byte("\x1b[F"),
		"cli", "send-text", "--no-paste", "--pane-id", paneArg); err != nil {
		return err
	}

	// 3. Send Enter
	_, _, err := runWeztermStdin(ctx, []byte(profile.Enter),
		"cli", "send-text", "--no-paste", "--pane-id", paneArg)
	return err
}

// SendEnterToPane sends the enter sequence for a shell type.
// Sends End key to move cursor to end of line, then Enter - as separate calls.
func SendEnterToPane(paneID int, shellType string) error {
	profile := GetShellProfile(shellType)
	paneArg := strconv.Itoa(paneID)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// 1. Send End key to move cursor to end of line
	if _, _, err := runWeztermStdin(ctx, []byte("\x1b[F"),
		"cli", "send-text", "--no-paste", "--pane-id", paneArg); err != nil {
		return err
	}

	// 2. Send Enter
	_, _, err := runWeztermStdin(ctx, []byte(profile.Enter),
		"cli", "send-text", "--no-paste", "--pane-id", paneArg)
	return err
}

// ClearLineInPane clears the current line in a pane using shell-specific sequence.
func ClearLineInPane(paneID int, shellType string) error {
	profile := GetShellProfile(shellType)
	if profile.ClearLine == "" {
		return nil // No clear line sequence for this shell
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cliArgs := []string{"cli", "send-text", "--no-paste", "--pane-id", strconv.Itoa(paneID)}
	_, _, err := runWeztermStdin(ctx, []byte(profile.ClearLine), cliArgs...)
	return err
}

// ActivatePane activates a specific pane in WezTerm.
func ActivatePane(paneID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	_, _, err := runWezterm(ctx, "cli", "activate-pane", "--pane-id", strconv.Itoa(paneID))
	return err
}

// FocusWezTermWindow brings the WezTerm window to the foreground (OS-specific).
func FocusWezTermWindow() error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("osascript", "-e", `tell application "WezTerm" to activate`).Run()
	case "windows":
		psScript := `
			Add-Type @"
				using System;
				using System.Runtime.InteropServices;
				public class Win32 {
					[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
					[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
				}
"@;
			$proc = Get-Process wezterm-gui -ErrorAction SilentlyContinue;
			if ($proc) {
				[Win32]::ShowWindow($proc.MainWindowHandle, 9);
				[Win32]::SetForegroundWindow($proc.MainWindowHandle);
			}
		`
		return exec.Command("powershell", "-Command", psScript).Run()
	default: // Linux
		if err := exec.Command("wmctrl", "-a", "wezterm").Run(); err != nil {
			return exec.Command("xdotool", "search", "--name", "wezterm", "windowactivate").Run()
		}
		return nil
	}
}

// PaneInfo contains basic pane information.
type PaneInfo struct {
	PaneID int    `json:"pane_id"`
	Title  string `json:"title"`
	CWD    string `json:"cwd"`
}

// FetchPaneList fetches the list of panes from WezTerm.
func FetchPaneList() ([]PaneInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	stdout, _, err := runWezterm(ctx, "cli", "list", "--format", "json")
	if err != nil {
		return nil, err
	}

	var panes []struct {
		PaneID int    `json:"pane_id"`
		Title  string `json:"title"`
		CWD    string `json:"cwd"`
	}
	if err := json.Unmarshal([]byte(stdout), &panes); err != nil {
		return nil, err
	}

	result := make([]PaneInfo, len(panes))
	for i, p := range panes {
		result[i] = PaneInfo{PaneID: p.PaneID, Title: p.Title, CWD: p.CWD}
	}
	return result, nil
}

// Errorf formats a tool error with stderr context.
func Errorf(tool, detail, stderr string, err error) string {
	if stderr != "" {
		return fmt.Sprintf("%s failed: %v\ndetail: %s\nstderr: %s", tool, err, detail, stderr)
	}
	return fmt.Sprintf("%s failed: %v\ndetail: %s", tool, err, detail)
}
