package util

import (
	"fmt"
	"strings"
)

// StringList is a list of strings and useful methods
type StringList []string

// Add a list of strings
func (s *StringList) Add(values ...string) {
	*s = append(*s, values...)
}

// Contains returns true if s contains value
func (s *StringList) Contains(value string) bool {
	for _, v := range *s {
		if v == value {
			return true
		}
	}
	return false
}

// Sprint adds a string using fmt.Sprint syntax
func (s *StringList) Sprint(a ...interface{}) {
	s.Add(fmt.Sprint(a...))
}

// Sprintf adds a string using fmt.Sprintf syntax
func (s *StringList) Sprintf(format string, a ...interface{}) {
	s.Add(fmt.Sprintf(format, a...))
}

// Join concatenates elements fo the list with given separator
func (s *StringList) Join(sep string) string {
	return strings.Join(*s, sep)
}
