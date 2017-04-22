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
	"log"
	"time"

	got "github.com/taskcluster/go-got"

	"github.com/taskcluster/taskcluster-worker/config"
)

type provider struct{}

func init() {
	config.Register("hostcredentials", provider{})
}

func (provider) Transform(cfg map[string]interface{}) error {
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
				log.Printf("Trying host-secrets server %s...", url)

				resp, err := g.Get(url).Send()
				if err != nil {
					log.Printf("result: %s; continuing to next server", err)
					continue
				}

				err = json.Unmarshal(resp.Body, &creds)
				if err != nil {
					log.Printf("decoding JSON from server: %s; continuing to next server", err)
					continue
				}

				retval := map[string]interface{}{
					"clientId":    creds.Credentials.ClientID,
					"accessToken": creds.Credentials.AccessToken,
				}
				if creds.Credentials.Certificate != "" {
					retval["certificate"] = creds.Credentials.Certificate
				}
				log.Printf("Success: host-secrets server gave clientId %s...", creds.Credentials.ClientID)
				return retval, nil
			}

			log.Printf("list of servers exhausted; sleeping before starting again")
			time.Sleep(60 * time.Second)
		}
	})
}
