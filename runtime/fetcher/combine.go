package fetcher

import (
	"fmt"

	schematypes "github.com/taskcluster/go-schematypes"
)

type fetcherSet struct {
	schema   schematypes.Schema
	fetchers []Fetcher
}

// Combine will a list of Fetchers into a single Fetcher implementation.
//
// For this to work well, the reference schemas should all be distrinct such
// that no reference matches more than one fetcher. If there is ambiguity the
// first Fetcher whose schema matches will be used.
func Combine(fetchers ...Fetcher) Fetcher {
	schema := schematypes.OneOf{}
	for _, f := range fetchers {
		schema = append(schema, f.Schema())
	}
	return &fetcherSet{schema, fetchers}
}

func (fs *fetcherSet) findFetcher(reference interface{}) (int, Fetcher) {
	for i, f := range fs.fetchers {
		if f.Schema().Validate(reference) == nil {
			return i, f
		}
	}
	// reference must validate against fs.Schema()!
	panic(fmt.Sprintf(
		"Reference: %#v doesn't validate against Fetcher.Schema()", reference,
	))
}

func (fs *fetcherSet) Schema() schematypes.Schema {
	return fs.schema
}

func (fs *fetcherSet) HashKey(reference interface{}) string {
	i, f := fs.findFetcher(reference)
	return fmt.Sprintf("%d:%s", i, f.HashKey(reference))
}

func (fs *fetcherSet) Scopes(reference interface{}) [][]string {
	_, f := fs.findFetcher(reference)
	return f.Scopes(reference)
}

func (fs *fetcherSet) Fetch(context Context, reference interface{}, target WriteSeekReseter) error {
	_, f := fs.findFetcher(reference)
	return f.Fetch(context, reference, target)
}
