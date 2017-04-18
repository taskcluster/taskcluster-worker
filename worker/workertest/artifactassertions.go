package workertest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// AnyArtifact creates an assertion that checks matches anything.
func AnyArtifact() func(t *testing.T, a Artifact) {
	// This is made a function that returns a function for consistency.
	return func(t *testing.T, a Artifact) {}
}

// GrepArtifact creates an assertion that holds if the artifact contains the
// given substring.
func GrepArtifact(substring string) func(t *testing.T, a Artifact) {
	return func(t *testing.T, a Artifact) {
		assert.Contains(t, string(a.Data), substring, "Expected substring in artifact: %s", a.Name)
	}
}

// S3Artifact creates an assertion that holds if the artifact is an S3 artifact
func S3Artifact() func(t *testing.T, a Artifact) {
	return func(t *testing.T, a Artifact) {
		assert.Equal(t, "s3", a.StorageType, "Expected storageType: 's3' for artifact: %s", a.Name)
	}
}

// ErrorArtifact creates an assertion that holds if the artifact is an error artifact
func ErrorArtifact() func(t *testing.T, a Artifact) {
	return func(t *testing.T, a Artifact) {
		assert.Equal(t, "error", a.StorageType, "Expected storageType: 'error' for artifact: %s", a.Name)
	}
}

// ReferenceArtifact creates an assertion that holds if the artifact is a reference artifact
func ReferenceArtifact() func(t *testing.T, a Artifact) {
	return func(t *testing.T, a Artifact) {
		assert.Equal(t, "reference", a.StorageType, "Expected storageType: 'reference' for artifact: %s", a.Name)
	}
}
