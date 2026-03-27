package textdiff

import (
	"strings"
)

// FindNewLines finds new lines in newBuf that appeared after the content in refLines.
// Both slices are fixed-position terminal snapshots: line[i] in refLines corresponds
// to line[i] in newBuf (same row offset from top of the fetched window).
// Walks forward positionally until the first line that differs — everything from
// that position onward in newBuf is considered new output.
// Returns the new lines and whether any positional anchor (shared prefix) was found.
func FindNewLines(refLines, newBuf []string, maxChars int) ([]string, bool) {
	if len(refLines) == 0 {
		// No reference - return all of newBuf as new
		return newBuf, true
	}
	if len(newBuf) == 0 {
		return nil, true
	}

	limit := len(refLines)
	if len(newBuf) < limit {
		limit = len(newBuf)
	}

	// Walk forward: find the first position where the two buffers diverge.
	divergeAt := -1
	for i := 0; i < limit; i++ {
		ref := refLines[i]
		cur := newBuf[i]
		if maxChars > 0 {
			if len(ref) > maxChars {
				ref = ref[:maxChars]
			}
			if len(cur) > maxChars {
				cur = cur[:maxChars]
			}
		}
		if ref != cur {
			divergeAt = i
			break
		}
	}

	if divergeAt == -1 {
		// refLines is a prefix of newBuf with no divergence within the compared range.
		// New content starts after the last compared line.
		newStart := limit
		if newStart >= len(newBuf) {
			return nil, true // Nothing new
		}
		return newBuf[newStart:], true
	}

	// Return everything from the first diverging line onward.
	return newBuf[divergeAt:], true
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
// Returns the truncated lines joined as a string, and the number of lines truncated.
func TruncateWithHint(lines []string, maxLines int) (string, int) {
	if len(lines) <= maxLines {
		return JoinLines(lines), 0
	}

	truncated := lines[:maxLines]
	truncatedCount := len(lines) - maxLines

	// Add hint at the end
	result := JoinLines(truncated)
	result += "\n\n... <more lines before, use 'wezterm cli get-text --start-line X --end-line Y' where X,Y are negative line numbers above the visible buffer to read remaining output>"

	return result, truncatedCount
}
