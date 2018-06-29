package relengapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type relengapiTokenJSON struct {
	Typ         string      `json:"typ"`
	ID          int         `json:"id,omitempty"`
	NotBefore   *time.Time  `json:"not_before,omitempty"`
	Expires     *time.Time  `json:"expires,omitempty"`
	Metadata    interface{} `json:"metadata,omitempty"`
	Disabled    bool        `json:"disabled,omitempty"`
	Permissions []string    `json:"permissions,omitempty"`
	Description string      `json:"description,omitempty"`
	User        string      `json:"user,omitempty"`
	Token       string      `json:"token,omitempty"`
}

func getTmpToken(url string, issuingToken string, expires time.Time, perms []string) (tok string, err error) {
	request := relengapiTokenJSON{
		Typ:         "tmp",
		Expires:     &expires,
		Permissions: perms,
		Metadata:    map[string]interface{}{},
	}

	reqbody, err := json.Marshal(request)
	if err != nil {
		return
	}

	client := &http.Client{}
	reqURL := fmt.Sprintf("%s/tokens", url)
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(reqbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", issuingToken))

	// Issuing a token is improbable to need a retry, but anyway better to
	// use httpbackoff package when it is fixed
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode != 200 {
		err = fmt.Errorf(
			"Got '%s' while trying to get new tmp token:\n%s",
			resp.Status, string(body))
		return
	}

	var responseBody interface{}
	err = json.Unmarshal(body, &responseBody)
	if err != nil {
		return
	}

	result := responseBody.(map[string]interface{})["result"]
	tok = result.(map[string]interface{})["token"].(string)
	return
}
