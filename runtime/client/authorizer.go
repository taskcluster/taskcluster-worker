package client

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/taskcluster/slugid-go/slugid"
	hawk "github.com/tent/hawk-go"
)

// An Authorizer is an interface for an object that can sign a request with
// taskcluster credentials.
type Authorizer interface {
	SignedHeader(method string, URL *url.URL, payload json.RawMessage) (string, error)
	SignURL(URL *url.URL, expiration time.Duration) (*url.URL, error)
	WithAuthorizedScopes(scopes ...string) Authorizer
}

type authorizer struct {
	getCredentials   func() (clientID, accessToken, certificate string, err error)
	authorizedScopes []string
}

// NewAuthorizer returns an authorizer that signs with credntials from getCredentials
func NewAuthorizer(getCredentials func() (clientID, accessToken, certificate string, err error)) Authorizer {
	return &authorizer{getCredentials, nil}
}

func (a *authorizer) SignedHeader(method string, URL *url.URL, payload json.RawMessage) (string, error) {
	clientID, accessToken, certificate, err := a.getCredentials()
	if err != nil {
		return "", err
	}
	auth := hawk.Auth{
		Method: method,
		Credentials: hawk.Credentials{
			ID:   clientID,
			Key:  accessToken,
			Hash: sha256.New,
		},
		Timestamp:  time.Now(),
		Nonce:      slugid.V4(),
		RequestURI: URL.RequestURI(),
		Host:       URL.Hostname(),
		Port:       URL.Port(),
	}

	// Set default port
	if auth.Port == "" {
		if URL.Scheme == "http" {
			auth.Port = "80"
		} else {
			auth.Port = "443"
		}
	}

	// Set ext if needed
	ext := make(map[string]interface{})
	if certificate != "" {
		ext["certificate"] = json.RawMessage(certificate)
	}
	if a.authorizedScopes != nil {
		ext["authorizedScopes"], _ = json.Marshal(a.authorizedScopes)
	}
	if len(ext) != 0 {
		data, _ := json.Marshal(ext)
		auth.Ext = base64.StdEncoding.EncodeToString(data)
	}

	// Set content hash, if content is provided
	if payload != nil {
		h := auth.PayloadHash("application/json")
		h.Write([]byte(payload))
		auth.SetHash(h)
	}

	// Return authorization header
	return auth.RequestHeader(), nil
}

func (a *authorizer) SignURL(URL *url.URL, expiration time.Duration) (*url.URL, error) {
	clientID, accessToken, certificate, err := a.getCredentials()
	if err != nil {
		return nil, err
	}
	auth := hawk.Auth{
		Method: http.MethodGet,
		Credentials: hawk.Credentials{
			ID:   clientID,
			Key:  accessToken,
			Hash: sha256.New,
		},
		Timestamp:  time.Now().Add(expiration),
		RequestURI: URL.RequestURI(),
		Host:       URL.Hostname(),
		Port:       URL.Port(),
	}

	// Set default port
	if auth.Port == "" {
		if URL.Scheme == "http" {
			auth.Port = "80"
		} else {
			auth.Port = "443"
		}
	}

	// Set ext if needed
	ext := make(map[string]interface{})
	if certificate != "" {
		ext["certificate"] = json.RawMessage(certificate)
	}
	if a.authorizedScopes != nil {
		ext["authorizedScopes"], _ = json.Marshal(a.authorizedScopes)
	}
	if len(ext) != 0 {
		data, _ := json.Marshal(ext)
		auth.Ext = base64.StdEncoding.EncodeToString(data)
	}

	// Add bewit to URL
	q := URL.Query()
	if q == nil {
		q = make(url.Values)
	}
	q.Set("bewit", auth.Bewit())
	u := *URL // don't modify URL given by pointer
	u.RawQuery = q.Encode()
	return &u, nil
}

func (a *authorizer) WithAuthorizedScopes(scopes ...string) Authorizer {
	if scopes == nil {
		scopes = []string{} // because nil, implies no authorizedScopes restriction
	}
	if a.authorizedScopes != nil {
		// Take intersection of scopes and a.authorizedScopes
		scopes = intersectScopes(scopes, a.authorizedScopes)
	}
	return &authorizer{
		getCredentials:   a.getCredentials,
		authorizedScopes: scopes,
	}
}

func intersectScopes(scopesA, scopesB []string) []string {
	hasScope := func(required string, scopes []string, exact bool) bool {
		for _, scope := range scopes {
			if required == scope {
				return exact
			}
			if strings.HasSuffix(scope, "*") && strings.HasPrefix(required, scope[0:len(scope)-1]) {
				return true
			}
		}
		return false
	}
	result := []string{}
	for _, scopeA := range scopesA {
		if !hasScope(scopeA, scopesB, true) {
			result = append(result, scopeA)
		}
	}
	for _, scopeB := range scopesB {
		if !hasScope(scopeB, scopesA, false) {
			result = append(result, scopeB)
		}
	}
	return result
}
