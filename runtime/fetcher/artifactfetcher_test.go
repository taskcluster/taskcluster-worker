package fetcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactFetcherScopes(t *testing.T) {
	assert.EqualValues(t, [][]string{[]string{}}, Artifact.Scopes(map[string]interface{}{
		"taskId":   "H6SAIKUFT2mewKH-qHzXjQ",
		"runId":    0,
		"artifact": "public/logs/live.log",
	}))
	assert.EqualValues(t, [][]string{[]string{"queue:get-artifact:private/logs/live.log"}}, Artifact.Scopes(map[string]interface{}{
		"taskId":   "H6SAIKUFT2mewKH-qHzXjQ",
		"runId":    0,
		"artifact": "private/logs/live.log",
	}))
}
