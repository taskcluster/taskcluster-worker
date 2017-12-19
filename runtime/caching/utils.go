package caching

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/pkg/errors"
)

// hashJSON takes interface{} as decoded by json.Unmarshal and returns a
// hash of the data with all keys sorted.
func hashJSON(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		panic(errors.Wrap(err, "expected data to be JSON serializable"))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
