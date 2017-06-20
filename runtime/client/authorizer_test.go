package client

import (
	"net/http"
	"net/url"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	got "github.com/taskcluster/go-got"
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

func testAuthorizer(secret string, clientScopes, requiredScopes, authorizedScopes []string, expectOk bool) func(*testing.T) {
	return func(t *testing.T) {
		u, _ := url.Parse("https://auth.taskcluster.net/v1/test-authenticate")
		a := NewAuthorizer(func() (string, string, string, error) {
			return "tester", secret, "", nil
		})
		if authorizedScopes != nil {
			a = a.WithAuthorizedScopes(authorizedScopes...)
		}
		g := got.New()
		req := g.Post(u.String(), nil)
		req.JSON(map[string]interface{}{
			"clientScopes":   clientScopes,
			"requiredScopes": requiredScopes,
		})
		authorization, err := a.SignHeader(req.Method, u, nil)
		assert.NoError(t, err, "SignHeader failed")
		req.Header.Set("Authorization", authorization)
		res, err := req.Send()
		if expectOk {
			assert.NoError(t, err, "request failed")
			if e, ok := err.(got.BadResponseCodeError); ok {
				t.Error(string(e.Body))
			}
			if err == nil {
				assert.Equal(t, http.StatusOK, res.StatusCode, "expected 200 ok")
			}
		} else {
			if _, ok := err.(got.BadResponseCodeError); !ok {
				assert.Fail(t, "Expected bad response code")
			}
		}
	}
}

func TestAuthorizer(t *testing.T) {
	t.Run("empty scopes", testAuthorizer("no-secret", []string{}, []string{}, nil, true))
	t.Run("a, b", testAuthorizer("no-secret", []string{"a", "b"}, []string{}, nil, true))
	t.Run("a required", testAuthorizer("no-secret", []string{"a", "b"}, []string{"a"}, nil, true))
	t.Run("aa cover by a*", testAuthorizer("no-secret", []string{"a*", "b"}, []string{"aa"}, nil, true))
	t.Run("authorizedScopes", testAuthorizer("no-secret", []string{"a", "b"}, []string{"a"}, []string{"a"}, true))
	t.Run("authorizedScopes w. aa*", testAuthorizer("no-secret", []string{"a*", "b"}, []string{"aa"}, []string{"aa*"}, true))
	t.Run("authorizedScopes fail", testAuthorizer("no-secret", []string{"a*", "b"}, []string{"aa"}, []string{"a"}, false))
	t.Run("a, b - wrong key", testAuthorizer("wrong-secret", []string{"a", "b"}, []string{}, nil, false))
}
