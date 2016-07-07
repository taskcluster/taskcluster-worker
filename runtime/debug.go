package runtime

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	prevDebugMessage = time.Now()
	debugLock        = sync.Mutex{}
	longestdebugName = 0
	nextColor        = 0
	colors           = []string{
		"34", // Blue
		"33", // Yellow
		"32", // Green
		"31", // Red
		"35", // Magenta
		"90", // Dark gray
		"93", // Light yellow
		"36", // Cyan
		"92", // Light green
		"91", // Light red
	}
)

var debugPattern = func() *regexp.Regexp {
	debug := os.Getenv("DEBUG")
	if debug == "" {
		return nil
	}
	// Credits: github.com/tj/go-debug
	debug = regexp.QuoteMeta(debug)
	debug = strings.Replace(debug, "\\*", ".*?", -1)
	debug = strings.Replace(debug, ",", "|", -1)
	debug = "^(" + debug + ")$"
	return regexp.MustCompile(debug)
}()

func debugDisabled(string, ...interface{}) {}

// Debug will return a debug(format, arg, arg...) function for which messages
// will be printed if the DEBUG environment variable is set.
//
// This is useful for development debugging only. Do not use this for messages
// that has any value in production.
func Debug(name string) func(string, ...interface{}) {
	if debugPattern == nil || !debugPattern.MatchString(name) {
		return debugDisabled
	}

	debugLock.Lock()
	defer debugLock.Unlock()

	// Pick a color
	color := colors[nextColor%len(colors)]
	nextColor++

	// Ensure we know the longest name
	if longestdebugName < len(name) {
		longestdebugName = len(name)
	}

	// Pad so that we align everything
	paddedName := name + strings.Repeat(" ", longestdebugName-len(name))
	if len(paddedName) != longestdebugName {
		panic("Internal error: len(paddedName) != longestdebugName")
	}

	return func(format string, args ...interface{}) {
		debugLock.Lock()
		now := time.Now()
		delay := now.Sub(prevDebugMessage)
		prevDebugMessage = now
		if len(paddedName) != longestdebugName {
			paddedName = name + strings.Repeat(" ", longestdebugName-len(name))
		}
		debugLock.Unlock()

		d := humanizeNano(delay.Nanoseconds())
		s := fmt.Sprintf(" %s \033[%sm\033[1m%s\033[21m\033[0m | ", d, color, paddedName)
		s += fmt.Sprintf(format, args...)
		fmt.Fprintln(os.Stderr, s)
	}
}

// Humanize nanoseconds to a string.
// Credits: github.com/tj/go-debug
func humanizeNano(n int64) string {
	suffix := "ns"
	color := "90" // dark grey
	switch {
	case n > 1000000000:
		n /= 1000000000
		suffix = "s"
		color = "31" // red
	case n > 1000000:
		n /= 1000000
		suffix = "ms"
		if n > 300 {
			color = "33" // yellow
		} else {
			color = "37" // light grey
		}
	case n > 1000:
		n /= 1000
		suffix = "us"
	}

	return fmt.Sprintf("\033[%sm%-6s\033[0m", color, strconv.Itoa(int(n))+suffix)
}
