package fetcher

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
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
	return &r, nil
}

func (u *urlHashReference) HashKey() string {
	if u.SHA512 != "" {
		return fmt.Sprintf("sha512=%s", strings.ToLower(u.SHA512))
	}
	if u.SHA256 != "" {
		return fmt.Sprintf("sha256=%s", strings.ToLower(u.SHA256))
	}
	// If we don't have sha256 or sha512 we cache based on the URL + hashes
	var k []string
	if u.SHA1 != "" {
		k = append(k, fmt.Sprintf("sha1=%s", strings.ToLower(u.SHA1)))
	}
	if u.MD5 != "" {
		k = append(k, fmt.Sprintf("md5=%s", strings.ToLower(u.MD5)))
	}
	k = append(k, fmt.Sprintf("url=%s", u.URL))
	return strings.Join(k, " ")
}

func (u *urlHashReference) Scopes() [][]string {
	return [][]string{{}} // Set containing the empty-scope-set
}

func (u *urlHashReference) Fetch(ctx Context, target WriteReseter) error {
	w := hashWriteReseter{
		Target: target,
	}
	if u.MD5 != "" {
		w.hashes = append(w.hashes, md5.New())
	}
	if u.SHA1 != "" {
		w.hashes = append(w.hashes, sha1.New())
	}
	if u.SHA256 != "" {
		w.hashes = append(w.hashes, sha256.New())
	}
	if u.SHA512 != "" {
		w.hashes = append(w.hashes, sha512.New())
	}
	err := fetchURLWithRetries(ctx, u.URL, u.URL, &w)
	if err != nil {
		return err
	}
	var h hash.Hash
	if u.MD5 != "" {
		h, w.hashes = w.hashes[0], w.hashes[1:]
		hashsum := hex.EncodeToString(h.Sum(nil))
		if u.MD5 != hashsum {
			return newBrokenReferenceError(u.URL, fmt.Sprintf(
				"did not match declared MD5, expected '%s', computed: '%s'",
				u.MD5, hashsum,
			))
		}
	}
	if u.SHA1 != "" {
		h, w.hashes = w.hashes[0], w.hashes[1:]
		hashsum := hex.EncodeToString(h.Sum(nil))
		if u.SHA1 != hashsum {
			return newBrokenReferenceError(u.URL, fmt.Sprintf(
				"did not match declared SHA1, expected '%s', computed: '%s'",
				u.SHA1, hashsum,
			))
		}
	}
	if u.SHA256 != "" {
		h, w.hashes = w.hashes[0], w.hashes[1:]
		hashsum := hex.EncodeToString(h.Sum(nil))
		if u.SHA256 != hashsum {
			return newBrokenReferenceError(u.URL, fmt.Sprintf(
				"did not match declared SHA256, expected '%s', computed: '%s'",
				u.SHA256, hashsum,
			))
		}
	}
	if u.SHA512 != "" {
		h, w.hashes = w.hashes[0], w.hashes[1:]
		hashsum := hex.EncodeToString(h.Sum(nil))
		if u.SHA512 != hashsum {
			return newBrokenReferenceError(u.URL, fmt.Sprintf(
				"did not match declared SHA512, expected '%s', computed: '%s'",
				u.SHA512, hashsum,
			))
		}
	}
	return nil
}

type hashWriteReseter struct {
	Target WriteReseter
	hashes []hash.Hash
}

func (w *hashWriteReseter) Write(p []byte) (n int, err error) {
	for _, h := range w.hashes {
		h.Write(p)
	}
	return w.Target.Write(p)
}

func (w *hashWriteReseter) Reset() error {
	for _, h := range w.hashes {
		h.Reset()
	}
	return w.Reset()
}
