package cache

import (
	"fmt"
	"io"
	"strings"
)

// cacheScope returns the scope required to use a cache with a given name
func cacheScope(name string) string {
	return fmt.Sprintf("worker:cache:%s", name)
}

// format scopeSets for usage in an error message.
//
// Scope-sets will be formatteded as: ['a', 'b', 'c'] or ['d', 'e']
func formatScopeSetRequirements(scopeSets [][]string) string {
	sets := make([]string, len(scopeSets))
	for i, scopes := range scopeSets {
		sets[i] = "'" + strings.Join(scopes, "', '") + "'"
	}
	return strings.Join(sets, " or ")
}

// errorCapturingReader will wrap a Reader such that it returns io.EOF instead
// of any error from the reader. Any errors from the reader will be assigned
// to the Err property.
type errorCapturingReader struct {
	Reader io.Reader
	Err    error
}

func (r *errorCapturingReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err != nil && err != io.EOF {
		r.Err = err
		err = io.EOF
	}
	return
}
