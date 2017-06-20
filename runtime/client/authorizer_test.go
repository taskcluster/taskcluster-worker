package client

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testScopeIntersection(scopesA, scopesB, scopesAB []string) func(*testing.T) {
	return func(t *testing.T) {
		sort.Strings(scopesAB)
		result := intersectScopes(scopesA, scopesB)
		sort.Strings(result)
		assert.EqualValues(t, scopesAB, result, "intersection failed")
		result = intersectScopes(scopesB, scopesA)
		sort.Strings(result)
		assert.EqualValues(t, scopesAB, result, "reversing failed")
	}
}

func TestIntersectScopes(t *testing.T) {
	t.Run("empty tests", testScopeIntersection([]string{}, []string{}, []string{}))

	t.Run("A + B = A,B", testScopeIntersection(
		[]string{"A"}, []string{"B"}, []string{"A", "B"},
	))
	t.Run("A + B,C = A,B,C", testScopeIntersection(
		[]string{"A"}, []string{"B", "C"}, []string{"A", "B", "C"},
	))
	t.Run("A,C + B,C = A,B,C", testScopeIntersection(
		[]string{"A", "C"}, []string{"B", "C"}, []string{"A", "B", "C"},
	))
	t.Run("a* + b* = a*,b*", testScopeIntersection(
		[]string{"a*"}, []string{"b*"}, []string{"a*", "b*"},
	))
	t.Run("a* + a* = a*", testScopeIntersection(
		[]string{"a*"}, []string{"a*"}, []string{"a*"},
	))
	t.Run("a* + ab* = a*", testScopeIntersection(
		[]string{"a*"}, []string{"ab*"}, []string{"a*"},
	))
	t.Run("ac* + ab* = ac*,ab*", testScopeIntersection(
		[]string{"ac*"}, []string{"ab*"}, []string{"ac*", "ab*"},
	))
	t.Run("ac*,ab + ab* = ac*,ab*", testScopeIntersection(
		[]string{"ac*", "ab"}, []string{"ab*"}, []string{"ac*", "ab*"},
	))
}
