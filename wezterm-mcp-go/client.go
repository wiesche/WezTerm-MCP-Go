package wezterm

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

const defaultTimeout = 10 * time.Second

// Platform holds OS-specific binary names.
type Platform struct {
	WezTermBin string
}

// DetectPlatform returns OS-specific configuration.
func DetectPlatform() Platform {
	if runtime.GOOS == "windows" {
		return Platform{WezTermBin: "wezterm.exe"}
	}
	return Platform{WezTermBin: "wezterm"}
}

var platform = DetectPlatform()

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
