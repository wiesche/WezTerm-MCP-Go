package paneops

import (
	"context"
	"strconv"
	"strings"
	"time"
	"wezterm-mcp-go/textdiff"
)

// TakeSnapshot fetches N lines from a pane and returns them as a slice.
func TakeSnapshot(paneID, lines int) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cliArgs := []string{"cli", "get-text", "--pane-id", strconv.Itoa(paneID)}
	if lines > 0 {
		cliArgs = append(cliArgs, "--start-line", strconv.Itoa(-lines))
	}

	stdout, _, err := runWezterm(ctx, cliArgs...)
	if err != nil {
		return nil, err
	}

	return textdiff.SplitLines(stdout), nil
}

// WaitAndSnapshot waits for the specified duration, then fetches lines from the pane.
// Returns the lines, elapsed time since start, and any error.
func WaitAndSnapshot(paneID int, waitMs int, fetchLines int, startTime time.Time) ([]string, time.Duration, error) {
	// Wait before reading
	if waitMs > 0 {
		time.Sleep(time.Duration(waitMs) * time.Millisecond)
	}

	lines, err := TakeSnapshot(paneID, fetchLines)
	elapsed := time.Since(startTime)

	return lines, elapsed, err
}

// CaptureOutput captures new output after a command execution.
// Takes a reference snapshot before the command, then compares with post-command snapshot.
// Returns new lines, elapsed time, and whether anchor was found.
func CaptureOutput(paneID int, refLines []string, waitMs int, maxNewLines, refWindow, maxChars int) ([]string, time.Duration, bool, error) {
	startTime := time.Now()

	// Fetch post-command buffer (need enough lines for matching + new output)
	fetchLines := maxNewLines + refWindow + 10 // extra buffer
	newLines, elapsed, err := WaitAndSnapshot(paneID, waitMs, fetchLines, startTime)
	if err != nil {
		return nil, elapsed, false, err
	}

	// Find new lines since reference
	newOutput, anchorFound := textdiff.FindNewLines(refLines, newLines, maxChars)

	return newOutput, elapsed, anchorFound, nil
}

// FormatOutputSnapshot formats the output snapshot for JSON response.
// Handles truncation and adds hint if needed.
func FormatOutputSnapshot(newLines []string, maxLines int) (string, int) {
	if len(newLines) == 0 {
		return "", 0
	}

	return textdiff.TruncateWithHint(newLines, maxLines)
}

// ShouldCaptureOutput determines if output capture should happen based on config and mode.
func ShouldCaptureOutput(userApproval bool, responseWaitMs int, manualMode bool, rejected bool) bool {
	// Don't capture in manual mode
	if manualMode {
		return false
	}
	// Don't capture if rejected
	if rejected {
		return false
	}
	// Capture if user_approval is true OR response_wait_ms > 0
	return userApproval || responseWaitMs > 0
}

// GetEffectiveWaitMs returns the effective wait time based on config.
func GetEffectiveWaitMs(userApproval bool, responseWaitMs int) int {
	if userApproval {
		// When user_approval is true, use the configured wait as minimum
		// (popup already blocked, so this is additional wait for output)
		if responseWaitMs > 0 {
			return responseWaitMs
		}
		// Default wait when user_approval but no explicit wait configured
		return 100 // Small default to let output appear
	}
	return responseWaitMs
}

// TrimLinesToCharLimit ensures no line exceeds maxChars for comparison purposes.
func TrimLinesToCharLimit(lines []string, maxChars int) []string {
	if maxChars <= 0 {
		return lines
	}
	result := make([]string, len(lines))
	for i, line := range lines {
		if len(line) > maxChars {
			result[i] = line[:maxChars]
		} else {
			result[i] = line
		}
	}
	return result
}

// CountNewlines counts the number of newlines in a string.
func CountNewlines(s string) int {
	return strings.Count(s, "\n")
}
