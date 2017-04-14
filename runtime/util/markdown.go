package util

import (
	"math"
	"regexp"
	"strings"
)

var whitespace = regexp.MustCompile(`^\s*$`)

// Markdown strips space indentation and replaces ' with `, allowing for markdown
// to be written as indended multi-line strings.
func Markdown(s string) string {
	lines := strings.Split(s, "\n")
	// Remove initial lines consisting of whitespace
	for len(lines) > 0 && whitespace.MatchString(lines[0]) {
		lines = lines[1:]
	}

	// Remove lines empty lines at the end
	for len(lines) > 0 && whitespace.MatchString(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}

	// Find common indentation
	I := math.MaxInt32
	for _, line := range lines {
		// Skip lines consisting of whitespace
		if whitespace.MatchString(line) {
			continue
		}
		// Find index of first non-TAB rune
		i := strings.IndexFunc(line, func(r rune) bool {
			return r != '\t'
		})
		if i < I {
			I = i
		}
	}
	// Cut lines to common indentation
	for i, line := range lines {
		if len(line)-1 < I {
			lines[i] = ""
			continue
		}
		lines[i] = line[I:]
	}

	return strings.Replace(strings.Join(lines, "\n"), "'", "`", -1)
}
