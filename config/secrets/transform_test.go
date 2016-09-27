package configsecrets

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taskcluster/taskcluster-worker/config/configtest"
)

func TestSecretsTransform(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/secret/my/super/secret" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{
      "expires": "2016-09-27T00:17:53.921Z",
      "secret": {"myKey": "hello-world"}
    }`))
	}))
	defer s.Close()

	configtest.Case{
		Transform: "secrets",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$secret": "my/super/secret", "key": "myKey"},
			"credentials": map[string]interface{}{
				"clientId":    "no-client",
				"accessToken": "no-secret",
			},
			"secretsBaseUrl": s.URL,
		},
		Result: map[string]interface{}{
			"key": "hello-world",
			"credentials": map[string]interface{}{
				"clientId":    "no-client",
				"accessToken": "no-secret",
			},
			"secretsBaseUrl": s.URL,
		},
	}.Test()
}

func TestSecretsTransformCertificate(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/secret/my/super/secret" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{
      "expires": "2016-09-27T00:17:53.921Z",
      "secret": {"myKey": "hello-world"}
    }`))
	}))
	defer s.Close()

	const cert = `{
    "version": 1,
    "scopes": [
      "*"
    ],
    "start": 1474934808198,
    "expiry": 1475194908198,
    "seed": "wQebYQTSRA6MzzOq9LSAAAAAOP8J8aQki5bdrCwJ5IxA",
    "signature": "QTSRA6MzzObAo+T/cDHxxE6SiuGHS0sAH2u4Z5b+kTA=",
    "issuer": "root"
  }`

	configtest.Case{
		Transform: "secrets",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$secret": "my/super/secret", "key": "myKey"},
			"credentials": map[string]interface{}{
				"clientId":    "no-client",
				"accessToken": "no-secret",
				"certificate": cert,
			},
			"secretsBaseUrl": s.URL,
		},
		Result: map[string]interface{}{
			"key": "hello-world",
			"credentials": map[string]interface{}{
				"clientId":    "no-client",
				"accessToken": "no-secret",
				"certificate": cert,
			},
			"secretsBaseUrl": s.URL,
		},
	}.Test()
}

func TestSecretsTransformComplex(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/secret/my/super/secret" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{
      "expires": "2016-09-27T00:17:53.921Z",
      "secret": {"myKey": {"key1": 1, "key2": [1,2,3]}}
    }`))
	}))
	defer s.Close()

	configtest.Case{
		Transform: "secrets",
		Input: map[string]interface{}{
			"key": map[string]interface{}{"$secret": "my/super/secret", "key": "myKey"},
			"credentials": map[string]interface{}{
				"clientId":    "no-client",
				"accessToken": "no-secret",
			},
			"secretsBaseUrl": s.URL,
		},
		Result: map[string]interface{}{
			"key": map[string]interface{}{
				"key1": float64(1),
				"key2": []interface{}{float64(1), float64(2), float64(3)},
			},
			"credentials": map[string]interface{}{
				"clientId":    "no-client",
				"accessToken": "no-secret",
			},
			"secretsBaseUrl": s.URL,
		},
	}.Test()
}
