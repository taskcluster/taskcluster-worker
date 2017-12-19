package cache

import "github.com/taskcluster/taskcluster-worker/runtime/fetcher"

type payloadEntry struct {
	Name       string      `json:"name"`
	MountPoint string      `json:"mountPoint"`
	Options    interface{} `json:"options"`
	Preload    interface{} `json:"preload"`
}

// A fetcher for pre-loading caches
var preloadFetcher = fetcher.Combine(
	// Allow fetching from URL
	fetcher.URL,
	// Allow fetching from queue artifacts
	fetcher.Artifact,
	// Allow fetching from queue referenced by index namespace
	fetcher.Index,
	// Allow fetching from URL + hash
	fetcher.URLHash,
)
