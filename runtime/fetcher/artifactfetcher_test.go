package fetcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArtifactFetcherScopesPublic(t *testing.T) {
	ctx := &fakeContext{Context: context.Background()}
	ref, err := Artifact.NewReference(ctx, map[string]interface{}{
		"taskId":   "H6SAIKUFT2mewKH-qHzXjQ",
		"runId":    0,
		"artifact": "public/logs/live.log",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, [][]string{{}}, ref.Scopes())
}

func TestArtifactFetcherScopesPrivate(t *testing.T) {
	ctx := &fakeContext{Context: context.Background()}
	ref, err := Artifact.NewReference(ctx, map[string]interface{}{
		"taskId":   "H6SAIKUFT2mewKH-qHzXjQ",
		"runId":    0,
		"artifact": "private/logs/live.log",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, [][]string{{"queue:get-artifact:private/logs/live.log"}}, ref.Scopes())
}
