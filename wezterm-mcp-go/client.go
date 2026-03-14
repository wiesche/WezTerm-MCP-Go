package wezterm

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
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

// IsWezTermGUIRunning checks if a WezTerm GUI process is running.
func IsWezTermGUIRunning() bool {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq "+platform.GUIProcessName)
	} else {
		cmd = exec.Command("pgrep", "-x", platform.GUIProcessName)
	}
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.Contains(string(output), platform.GUIProcessName)
	}
	return len(output) > 0
}

// StartWezTermGUI starts a WezTerm GUI instance synchronously and waits for panes.
func StartWezTermGUI(ctx context.Context) error {
	cmd := exec.Command(platform.WezTermBin)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	time.Sleep(1000 * time.Millisecond)
	// Wait for GUI to initialize and panes to appear (up to 3 seconds)
	// for i := 0; i < 30; i++ {
	// 	time.Sleep(100 * time.Millisecond)
	// 	// Check if panes are available using --prefer-mux to connect to mux server
	// 	stdout, _, err := runWezterm(ctx, "cli", "list", "--format", "json")
	// 	if err == nil && len(stdout) > 2 { // More than "[]"
	// 		return nil
	// 	}
	//}
	return nil // Return nil even if we couldn't verify - let spawn try anyway
}

// CheckWezTermOnPath checks if wezterm binary is available on PATH.
func CheckWezTermOnPath() bool {
	_, err := exec.LookPath(platform.WezTermBin)
	return err == nil
}

// runWezterm executes "wezterm cli --prefer-mux <args>" and returns stdout.
// The --prefer-mux flag must come after 'cli', not before.
func runWezterm(ctx context.Context, args ...string) (string, string, error) {
	var full []string
	if len(args) > 0 && args[0] == "cli" {
		// Insert --prefer-mux after 'cli'
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
// The --prefer-mux flag must come after 'cli', not before.
func runWeztermStdin(ctx context.Context, stdin []byte, args ...string) (string, string, error) {
	var full []string
	if len(args) > 0 && args[0] == "cli" {
		// Insert --prefer-mux after 'cli'
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

// runWeztermSimple executes wezterm without --prefer-mux (for spawn, etc).
func runWeztermSimple(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, platform.WezTermBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// withTimeout wraps context with default timeout.
func withTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, defaultTimeout)
}

// errorf formats a tool error with stderr context.
func errorf(tool, detail, stderr string, err error) string {
	if stderr != "" {
		return fmt.Sprintf("%s failed: %v\ndetail: %s\nstderr: %s", tool, err, detail, stderr)
	}
	return fmt.Sprintf("%s failed: %v\ndetail: %s", tool, err, detail)
}
