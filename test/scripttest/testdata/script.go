// package main contains an example script that could be used with
// script-engine. This example is used in CI for integration tests.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"
)

func main() {
	// Read payload from stdin
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read from stdin, err: %s\n", err)
		os.Exit(4) // contract violation exit with fatal internal error
	}

	// struct matches the expected input given the schema from config
	var payload struct {
		Delay     int               `json:"delay"`
		Message   string            `json:"message"`
		Result    string            `json:"result"`
		Artifacts map[string]string `json:"artifacts"`
		URL       string            `json:"url"`
	}
	// parse stdin as JSON
	err = json.Unmarshal(data, &payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse stdin as JSON, err: %s\n", err)
		os.Exit(4) // contract violation exit with fatal internal error
	}

	// Sleep given delay
	fmt.Printf("script started processing task, by sleeping %d ms\n", payload.Delay)
	time.Sleep(time.Duration(payload.Delay) * time.Millisecond)

	// Print message
	fmt.Println(payload.Message)

	// Get URL if one was given
	if payload.URL != "" {
		var res *http.Response
		res, err = http.Get(payload.URL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to fetch url: %s, err: %s\n", payload.URL, err)
			os.Exit(1) // fail the task
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "Got status %d from url: %s\n", res.StatusCode, payload.URL)
		}
		_, err = io.Copy(os.Stdout, res.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read response from url: %s, err: %s\n", payload.URL, err)
			os.Exit(1) // fail the task
		}
	}

	// Create artifacts
	for name, value := range payload.Artifacts {
		err = os.MkdirAll(path.Join(".", "artifacts", path.Dir(name)), 0700)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create folders for artifact: '%s', err: %s\n", name, err)
			os.Exit(4)
		}
		err = ioutil.WriteFile(path.Join(".", "artifacts", name), []byte(value), 0600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create file for artifact: '%s', err: %s\n", name, err)
			os.Exit(4)
		}
	}

	// Exit with requested result, so we can test the worker
	switch payload.Result {
	case "pass":
		fmt.Fprintf(os.Stderr, "task is passing\n")
		os.Exit(0)
	case "fail":
		fmt.Fprintf(os.Stderr, "task is failing\n")
		os.Exit(1)
	case "malformed-payload":
		fmt.Fprintf(os.Stderr, "triggering a malformed-payload error\n")
		os.Exit(2)
	case "non-fatal-error":
		fmt.Fprintf(os.Stderr, "triggering a non-fatal-error for testing\n")
		os.Exit(3)
	default:
		// This shouldn't be possible, if we specify the right schema
		fmt.Fprintf(os.Stderr, "unknown result '%s' requested\n", payload.Result)
		os.Exit(4)
	}
}
