package client

import (
	"net/http"
	"net/url"
	"sort"
	"testing"
	"time"

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

func testAuthorizerSignHeader(secret string, clientScopes, requiredScopes, authorizedScopes []string, expectOk bool) func(*testing.T) {
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

func TestAuthorizerSignHeader(t *testing.T) {
	t.Run("empty scopes", testAuthorizerSignHeader("no-secret", []string{}, []string{}, nil, true))
	t.Run("a, b", testAuthorizerSignHeader("no-secret", []string{"a", "b"}, []string{}, nil, true))
	t.Run("a required", testAuthorizerSignHeader("no-secret", []string{"a", "b"}, []string{"a"}, nil, true))
	t.Run("aa cover by a*", testAuthorizerSignHeader("no-secret", []string{"a*", "b"}, []string{"aa"}, nil, true))
	t.Run("authorizedScopes", testAuthorizerSignHeader("no-secret", []string{"a", "b"}, []string{"a"}, []string{"a"}, true))
	t.Run("authorizedScopes w. aa*", testAuthorizerSignHeader("no-secret", []string{"a*", "b"}, []string{"aa"}, []string{"aa*"}, true))
	t.Run("authorizedScopes fail", testAuthorizerSignHeader("no-secret", []string{"a*", "b"}, []string{"aa"}, []string{"a"}, false))
	t.Run("a, b - wrong key", testAuthorizerSignHeader("wrong-secret", []string{"a", "b"}, []string{}, nil, false))
}

func testAuthorizerSignURL(secret string, authorizedScopes []string, expectOk bool) func(*testing.T) {
	return func(t *testing.T) {
		u, _ := url.Parse("https://auth.taskcluster.net/v1/test-authenticate-get")
		a := NewAuthorizer(func() (string, string, string, error) {
			return "tester", secret, "", nil
		})
		if authorizedScopes != nil {
			a = a.WithAuthorizedScopes(authorizedScopes...)
		}
		u2, err := a.SignURL(u, 15*time.Minute)
		assert.NoError(t, err, "SignURL failed")
		g := got.New()
		res, err := g.Get(u2.String()).Send()
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

func TestAuthorizerSignURL(t *testing.T) {
	t.Run("simple", testAuthorizerSignURL("no-secret", nil, true))
	t.Run("wrong secret", testAuthorizerSignURL("wrong-secret", nil, false))
	t.Run("authorizedScopes", testAuthorizerSignURL("no-secret", []string{"test:auth*"}, true))
}
