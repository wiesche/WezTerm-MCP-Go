package textdiff

import (
	"hash/fnv"
	"strings"
)

// LineHash computes a fast hash for a line (truncated to maxChars).
func LineHash(line string, maxChars int) uint64 {
	if len(line) > maxChars {
		line = line[:maxChars]
	}
	h := fnv.New64a()
	h.Write([]byte(line))
	return h.Sum64()
}

// hashLines computes hashes for all lines in a slice.
func hashLines(lines []string, maxChars int) []uint64 {
	hashes := make([]uint64, len(lines))
	for i, line := range lines {
		hashes[i] = LineHash(line, maxChars)
	}
	return hashes
}

// FindNewLines finds new lines in newBuf that appeared after the content in refLines.
// Uses a sliding window suffix match algorithm (Boyer-Moore-Horspool variant at line granularity).
// Returns the new lines and whether an anchor was found.
func FindNewLines(refLines, newBuf []string, maxChars int) ([]string, bool) {
	if len(refLines) == 0 {
		// No reference - return all of newBuf as new
		return newBuf, true
	}
	if len(newBuf) == 0 {
		return nil, true
	}

	refHashes := hashLines(refLines, maxChars)
	newHashes := hashLines(newBuf, maxChars)

	refLen := len(refHashes)
	newLen := len(newHashes)

	// Slide the reference window over the new buffer from end backwards
	// We want to find where refLines matches in newBuf
	for startPos := newLen - refLen; startPos >= 0; startPos-- {
		// Check if refLines matches at this position
		match := true
		for i := 0; i < refLen; i++ {
			if refHashes[i] != newHashes[startPos+i] {
				match = false
				break
			}
		}

		if match {
			// Found anchor at startPos
			// New lines are everything after startPos + refLen
			newStart := startPos + refLen
			if newStart >= newLen {
				return nil, true // No new lines
			}
			return newBuf[newStart:], true
		}
	}

	// No anchor found - the buffer changed significantly
	// Return last portion of newBuf as a best-effort
	return nil, false
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
