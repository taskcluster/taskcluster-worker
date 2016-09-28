// Package configsecrets implements a TransformationProvider that replaces
// objects on the form: {$secret: "NAME", key: "KEY"} with the value of the
// key "KEY" taken from the secret NAME loaded from taskcluster-secrets.
//
// This transformation will fail if the configuration object doesn't contain
// valid taskcluster credentials in the 'credentials' property.
package configsecrets

import (
	"encoding/json"
	"errors"

	"github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/secrets"
	"github.com/taskcluster/taskcluster-worker/config"
)

type provider struct{}

func init() {
	config.Register("secrets", provider{})
}

func (provider) Transform(config map[string]interface{}) error {
	c, ok := config["credentials"].(map[string]interface{})
	if !ok {
		return errors.New("Expected 'credentials' property to hold credentials")
	}
	creds := &tcclient.Credentials{}
	creds.ClientID, _ = c["clientId"].(string)
	creds.AccessToken, _ = c["accessToken"].(string)
	creds.Certificate, _ = c["certificate"].(string)
	if creds.ClientID == "" || creds.AccessToken == "" {
		return errors.New("Expected properties: credentials.clientId and credentials.accessToken")
	}

	// Create a secrets client
	s := secrets.New(creds)

	// Overwrite the baseUrl for secrets if one is given
	if baseURL, _ := config["secretsBaseUrl"].(string); baseURL != "" {
		s.BaseURL = baseURL
	}

	// Create a cache to avoid loading the same secret twice, we use the same
	// creds for all calls and we don't persistent the cache so there is no risk
	// of scope elevation here.
	cache := map[string]map[string]interface{}{}

	_, err := injectSecrets(s, cache, config)
	return err
}

func injectSecrets(
	s *secrets.Secrets,
	cache map[string]map[string]interface{},
	val interface{},
) (interface{}, error) {
	switch val := val.(type) {
	case []interface{}:
		for i, v := range val {
			v, err := injectSecrets(s, cache, v)
			val[i] = v
			if err != nil {
				return nil, err
			}
		}
	case map[string]interface{}:
		name, ok1 := val["$secret"].(string)
		key, ok2 := val["key"].(string)
		if ok1 && ok2 && len(val) == 2 {
			// If secret isn't in the cache we try to load it
			if _, ok := cache[name]; !ok {
				secret, err := s.Get(name)
				if err != nil {
					return nil, err
				}
				value := map[string]interface{}{}
				_ = json.Unmarshal(secret.Secret, &value)
				cache[name] = value
			}
			// Get secret from cache
			return cache[name][key], nil
		}
		for k, v := range val {
			v, err := injectSecrets(s, cache, v)
			if err != nil {
				return nil, err
			}
			val[k] = v
		}
	}
	return val, nil
}
