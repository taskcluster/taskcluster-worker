package fetcher

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type urlHashFetcher struct{}

type urlHashReference struct {
	URL    string `json:"url"`
	MD5    string `json:"md5"`
	SHA1   string `json:"sha1"`
	SHA256 string `json:"sha256"`
	SHA512 string `json:"sha512"`
}

// URLHash is Fetcher for downloading files from a URL with a given hash
var URLHash Fetcher = urlHashFetcher{}

var urlHashSchema schematypes.Schema = schematypes.Object{
	Title: "Fetch from URL with Hash",
	Description: util.Markdown(`
		Fetch resource from a URL and validate against given hash.

		Hash must be specified in hexadecimal notation, you may specify none or all
		of 'md5', 'sha1', 'sha256', or 'sha512', all specified hashes will be
		validated. If no hash is specified, no validation will be done.
	`),
	Properties: schematypes.Properties{
		"url": schematypes.URI{
			Title:       "URL",
			Description: "URL to fetch resource from, this must be `http://` or `https://`.",
		},
		"md5": schematypes.String{
			Title:   "MD5 as hex",
			Pattern: `^[0-9a-fA-F]{32}$`,
		},
		"sha1": schematypes.String{
			Title:   "SHA1 as hex",
			Pattern: `^[0-9a-fA-F]{40}$`,
		},
		"sha256": schematypes.String{
			Title:   "SHA256 as hex",
			Pattern: `^[0-9a-fA-F]{64}$`,
		},
		"sha512": schematypes.String{
			Title:   "SHA512 as hex",
			Pattern: `^[0-9a-fA-F]{128}$`,
		},
	},
	Required: []string{"url"},
}

func (urlHashFetcher) Schema() schematypes.Schema {
	return urlHashSchema
}

func (urlHashFetcher) NewReference(ctx Context, options interface{}) (Reference, error) {
	var r urlHashReference
	schematypes.MustValidateAndMap(urlHashSchema, options, &r)
	// Normalize to lower case for sanity
	r.MD5 = strings.ToLower(r.MD5)
	r.SHA1 = strings.ToLower(r.SHA1)
	r.SHA256 = strings.ToLower(r.SHA256)
	r.SHA512 = strings.ToLower(r.SHA512)
	return &r, nil
}

func (u *urlHashReference) HashKey() string {
	if u.SHA512 != "" {
		return fmt.Sprintf("sha512=%s", u.SHA512)
	}
	if u.SHA256 != "" {
		return fmt.Sprintf("sha256=%s", u.SHA256)
	}
	// If we don't have sha256 or sha512 we cache based on the URL + hashes
	var k []string
	if u.SHA1 != "" {
		k = append(k, fmt.Sprintf("sha1=%s", u.SHA1))
	}
	if u.MD5 != "" {
		k = append(k, fmt.Sprintf("md5=%s", u.MD5))
	}
	k = append(k, fmt.Sprintf("url=%s", u.URL))
	return strings.Join(k, " ")
}

func (u *urlHashReference) Scopes() [][]string {
	return [][]string{{}} // Set containing the empty-scope-set
}

func (u *urlHashReference) Fetch(ctx Context, target WriteReseter) error {
	// Create  a list of hash sums we want to compute
	var hashers []hash.Hash // hash implementation for computation of hash
	var hashalg []string    // hash algorithm for error messages
	var hashsum []string    // hash sum expected for validation
	if u.MD5 != "" {
		hashers = append(hashers, md5.New())
		hashalg = append(hashalg, "MD5")
		hashsum = append(hashsum, u.MD5)
	}
	if u.SHA1 != "" {
		hashers = append(hashers, sha1.New())
		hashalg = append(hashalg, "SHA1")
		hashsum = append(hashsum, u.SHA1)
	}
	if u.SHA256 != "" {
		hashers = append(hashers, sha256.New())
		hashalg = append(hashalg, "SHA256")
		hashsum = append(hashsum, u.SHA256)
	}
	if u.SHA512 != "" {
		hashers = append(hashers, sha512.New())
		hashalg = append(hashalg, "SHA512")
		hashsum = append(hashsum, u.SHA512)
	}

	// Fetch from URL while computing hashes
	if err := fetchURLWithRetries(ctx, u.URL, u.URL, &hashWriteReseter{target, hashers}); err != nil {
		return err
	}

	// For each hasher we compute hash sum and compare to the target
	for i, h := range hashers {
		sum := strings.ToLower(hex.EncodeToString(h.Sum(nil)))
		if subtle.ConstantTimeCompare([]byte(hashsum[i]), []byte(sum)) != 1 {
			target.Reset()
			return newBrokenReferenceError(u.URL, fmt.Sprintf(
				"did not match declared %s, expected '%s', computed: '%s'",
				hashalg[i], hashsum[i], sum,
			))
		}
	}
	return nil
}

type hashWriteReseter struct {
	Target  WriteReseter
	hashers []hash.Hash
}

func (w *hashWriteReseter) Write(p []byte) (n int, err error) {
	for _, h := range w.hashers {
		h.Write(p)
	}
	return w.Target.Write(p)
}

func (w *hashWriteReseter) Reset() error {
	for _, h := range w.hashers {
		h.Reset()
	}
	return w.Target.Reset()
}
