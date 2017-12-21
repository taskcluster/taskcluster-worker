// Package hostcredentials implements a TransformationProvider that fetches
// credentials from the (oddly named) `host-secrets` service and replaces
// objects of the form: {$hostcredentials: [url, url]} with the credentials.
//
// The given URLs should point to the `/v1/credentials` endpoint of instances of the
// [taskcluster-host-secrets](https://github.com/taskcluster/taskcluster-host-secrets)
// service.  They will be tried in order until success.  This is a simple form
// of client-side resilience to failure of a single instance.
//
// Note that this transform will need to run before the "hostsecrets" transform.
package hostcredentials

import (
	"encoding/json"
	"time"

	got "github.com/taskcluster/go-got"

	"github.com/taskcluster/taskcluster-worker/config"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type provider struct{}

func init() {
	config.Register("hostcredentials", provider{})
}

func (provider) Transform(cfg map[string]interface{}, monitor runtime.Monitor) error {
	g := got.New()

	return config.ReplaceObjects(cfg, "hostcredentials", func(val map[string]interface{}) (interface{}, error) {
		var urls []string
		for _, u := range val["$hostcredentials"].([]interface{}) {
			urls = append(urls, u.(string))
		}

		var creds struct {
			Credentials struct {
				ClientID    string `json:"clientId"`
				AccessToken string `json:"accessToken"`
				Certificate string `json:"certificate"`
			} `json:"credentials"`
		}

		for {
			for _, url := range urls {
				monitor.Info("Trying host-secrets server ", url)

				resp, err := g.Get(url).Send()
				if err != nil {
					monitor.ReportError(err, "error fetching secrets; continuing to next server")
					continue
				}

				err = json.Unmarshal(resp.Body, &creds)
				if err != nil {
					monitor.ReportError(err, "error decoding JSON from server; continuing to next server")
					continue
				}

				retval := map[string]interface{}{
					"clientId":    creds.Credentials.ClientID,
					"accessToken": creds.Credentials.AccessToken,
				}
				if creds.Credentials.Certificate != "" {
					retval["certificate"] = creds.Credentials.Certificate
				}
				monitor.Info("Success: host-secrets server gave clientId ", creds.Credentials.ClientID)
				return retval, nil
			}

			monitor.Info("list of servers exhausted; sleeping before starting again")
			time.Sleep(60 * time.Second)
		}
	})
}
