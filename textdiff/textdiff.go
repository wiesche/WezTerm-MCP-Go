package textdiff

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// getViewportHeight fetches the visible viewport height by calling wezterm cli get-text
// without --start-line, which returns the full visible screen. The line count gives viewport height.
func getViewportHeight(paneID int) int {
	var weztermBin string
	if runtime.GOOS == "windows" {
		weztermBin = "wezterm.exe"
	} else {
		weztermBin = "wezterm"
	}

	cmd := exec.Command(weztermBin, "cli", "get-text", "--pane-id", fmt.Sprintf("%d", paneID))
	output, err := cmd.Output()
	if err != nil {
		return 24 // Default fallback
	}

	lines := SplitLines(string(output))
	return len(lines)
}

// FindNewLines finds new lines in newBuf that appeared after the content in refLines.
// Uses bottom-to-top comparison: searches for the longest suffix of refLines that matches
// a contiguous block in newBuf, starting from the end and working backwards.
// This is efficient when refLines is smaller than newBuf (common case: ref is 20 lines,
// newBuf may have 50+ lines of new output).
// paneID is used to generate a hint when no anchor is found (lines scrolled off viewport).
// Returns the new lines and whether an anchor was found.
func FindNewLines(refLines, newBuf []string, maxChars int, paneID int) ([]string, bool) {
	if len(refLines) == 0 {
		// No reference - return all of newBuf as new
		return newBuf, true
	}
	if len(newBuf) == 0 {
		return nil, true
	}

	refLen := len(refLines)
	newLen := len(newBuf)

	// Helper to compare two lines with maxChars truncation
	linesEqual := func(a, b string) bool {
		if maxChars > 0 {
			if len(a) > maxChars {
				a = a[:maxChars]
			}
			if len(b) > maxChars {
				b = b[:maxChars]
			}
		}
		return a == b
	}

	// adds a hint line before a returned text buffer
	appendHintBefore := func(buf []string) []string {
		viewportHeight := getViewportHeight(paneID)
		bufLen := len(buf)

		// Calculate lines above visible output: viewport_height - bufLen + 1
		// This gives a negative value when content is in scrollback
		endLine := viewportHeight - bufLen + 1

		// Only add hint if there are lines above (endLine is negative)
		if endLine < 0 {
			//startLine := endLine - 5 // Show 5 extra lines above for context
			hint := fmt.Sprintf("...< retrieve lines above using 'wezterm cli get-text --pane-id %d --start-line X --end-line %d' where X < %d> ", paneID, endLine, endLine)
			return append([]string{hint}, buf...)
		}
		return buf
	}

	// Match the lines in newBuf against the lines in refLines by searching for the last line in refLines in newBuf in reverse order
	reverse_ref_index := refLen - 2
	for running_index := newLen - 1; running_index >= 0; running_index-- {
		// loop for matching the whole reflines buffer ffrom back to fromt
		//for reverse_ref_index := len(refLines)-1; reverse_ref_index >0; reverse_ref_index-- {
		reverse_ref_index = refLen - 2 // Count over from line above the matching as the last line might have changed by the user input. Resets matching.
		for temp_running_index := running_index; temp_running_index >= 0; temp_running_index-- {
			line := newBuf[temp_running_index]
			matchline := refLines[reverse_ref_index]
			if !linesEqual(line, matchline) {
				break
			}

			// if reverse_ref_index < 0 { // Case: the entire reference matched -> all text after the running_index + len(refLines) is new
			// 	return newBuf[temp_running_index+len(refLines)-1:], true
			// }
			if temp_running_index == 0 || reverse_ref_index == 0 { //Case: if  reverse_ref_index > 0, the reference window partially overlaps with the end of the new buffer. All text after ref_index is new
				if temp_running_index == 0 || reverse_ref_index == refLen-2 {
					// All of newBuf is new - prepend hint and return
					return appendHintBefore(newBuf), true
				}
				return newBuf[temp_running_index+refLen-reverse_ref_index-1:], true
			}
			reverse_ref_index--
		}
		//Case: all of the new buffer is new and gets returned, the old reference buffer got pushed out of the matching window. Insert a hint to retreive above text using Wezterm in place of the first line of the new buffer in that case.

	}

	// No anchor found - buffer changed completely
	return appendHintBefore(newBuf), false
}

// SplitLines splits text into lines, preserving empty lines.
func SplitLines(text string) []string {
	if text == "" {
		return nil
	}
	// Split by newlines, but keep trailing newline info
	lines := strings.Split(text, "\n")
	// Remove trailing empty string if text ended with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// JoinLines joins lines back into text with newlines.
func JoinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}
