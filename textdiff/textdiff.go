package textdiff

import (
	"fmt"
	"strings"
)

// FindNewLines finds new lines in newBuf that appeared after the content in refLines.
// Uses bottom-to-top comparison: searches for the longest suffix of refLines that matches
// a contiguous block in newBuf, starting from the end and working backwards.
// This is efficient when refLines is smaller than newBuf (common case: ref is 20 lines,
// newBuf may have 50+ lines of new output).
// Returns the new lines and whether an anchor was found.
func FindNewLines(refLines, newBuf []string, maxChars int) ([]string, bool) {
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
	// Match the lines in newBuf against the lines in refLines by searching for the last line in refLines in newBuf in reverse order

	for running_index := newLen - 1; running_index >= 0; running_index-- {
		// loop for matching the whole reflines buffer ffrom back to fromt
		//for reverse_ref_index := len(refLines)-1; reverse_ref_index >0; reverse_ref_index-- {
		reverse_ref_index := refLen - 2 // Count over from line above the matching as the last line might have changed by the user input. Resets matching.
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
				return newBuf[temp_running_index+refLen-reverse_ref_index-1:], true
			}
			reverse_ref_index--
		}
		//Case: all of the new buffer is new and gets returned, the old reference buffer got pushed out of the matching window. Insert a hint to retreive above text using Wezterm in place of the first line of the new buffer in that case.

	}

	// No anchor found - buffer changed completely
	return newBuf, false
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

// TruncateWithHint truncates lines to maxLines and adds a hint for remaining content.
// The hint appears at the BEGINNING of the output with the actual line number to use.
// Returns the truncated lines joined as a string, and the number of lines truncated.
func TruncateWithHint(lines []string, maxLines int) (string, int) {
	if len(lines) <= maxLines {
		return JoinLines(lines), 0
	}

	truncatedCount := len(lines) - maxLines

	// The hint shows how to read the truncated lines above the visible output.
	// Y = -maxLines means "maxLines lines above the visible buffer start"
	hint := fmt.Sprintf("...<%d more lines above. Use 'wezterm cli get-text --start-line %d --end-line -1'>\n\n", truncatedCount, -truncatedCount)

	result := hint + JoinLines(lines[:maxLines])
	return result, truncatedCount
}
