package fetcher

import (
	"fmt"

	schematypes "github.com/taskcluster/go-schematypes"
)

type fetcherSet struct {
	schema   schematypes.Schema
	fetchers []Fetcher
}

// Combine a list of Fetchers into a single Fetcher implementation.
//
// For this to work well, the reference schemas should all be distinct such
// that no reference matches more than one fetcher. If there is ambiguity the
// first Fetcher whose schema matches will be used.
func Combine(fetchers ...Fetcher) Fetcher {
	schema := schematypes.OneOf{}
	for _, f := range fetchers {
		schema = append(schema, f.Schema())
	}
	return &fetcherSet{schema, fetchers}
}

func (fs *fetcherSet) findFetcher(options interface{}) (int, Fetcher) {
	for i, f := range fs.fetchers {
		if f.Schema().Validate(options) == nil {
			return i, f
		}
	}
	// options must validate against fs.Schema()!
	panic(fmt.Sprintf(
		"Reference: %#v doesn't validate against Fetcher.Schema()", options,
	))
}

func (fs *fetcherSet) Schema() schematypes.Schema {
	return fs.schema
}

type wrappedReference struct {
	Reference
	index int // Used to prefix HashKey so that hash-keys won't collide across fetchers
}

func (w *wrappedReference) HashKey() string {
	return fmt.Sprintf("%d:%s", w.index, w.Reference.HashKey())
}

func (fs *fetcherSet) NewReference(ctx Context, options interface{}) (Reference, error) {
	i, f := fs.findFetcher(options)
	ref, err := f.NewReference(ctx, options)
	if err != nil {
		return nil, err
	}
	return &wrappedReference{Reference: ref, index: i}, nil
}
