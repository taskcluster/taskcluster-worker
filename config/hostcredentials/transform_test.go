package hostcredentials

import (
	"net/http"
	"net/http/httptest"
	"testing"

	//"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/config/configtest"
)

func TestHostCredentialsTransform(t *testing.T) {
	bad1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer bad1.Close()

	bad2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{invalid json!!!}`))
	}))
	defer bad2.Close()

	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/h-s/v1/credentials" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{
      "clientId": "clid",
      "accessToken": "atat",
      "certificate": "{cert}"
    }`))
	}))
	defer good.Close()

	configtest.Case{
		Transform: "hostcredentials",
		Input: map[string]interface{}{
			"creds": map[string]interface{}{
				"$hostcredentials": []interface{}{
					bad1.URL + "/h-s/v1/credentials",
					bad2.URL + "/h-s/v1/credentials",
					good.URL + "/h-s/v1/credentials",
				},
			},
		},
		Result: map[string]interface{}{
			"creds": map[string]interface{}{
				"clientId":    "clid",
				"accessToken": "atat",
				"certificate": "{cert}",
			},
		},
	}.Test(t)
}
