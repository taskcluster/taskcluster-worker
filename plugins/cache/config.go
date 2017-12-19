package cache

import (
	"time"

	schematypes "github.com/taskcluster/go-schematypes"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

type config struct {
	MaxPurgeCacheDelay time.Duration `json:"maxPurgeCacheDelay"`
	PurgeCacheBaseURL  string        `json:"purgeCacheBaseUrl"`
}

var configSchema = schematypes.Object{
	Title: "Cache Plugin",
	Description: util.Markdown(`
		Configuration for the cache plugin that manages sandbox caches.
	`),
	Properties: schematypes.Properties{
		"maxPurgeCacheDelay": schematypes.Duration{
			Title: "Maximum Cache Purge Delay",
			Description: util.Markdown(`
				The cache plugin will call the taskcluster-purge-cache service to fetch
				_purge-cache requests_, these are request that caches should be purged.

				The cache plugin will pull the taskcluster-purge-cache service before
				every task to ensure that caches requested to be purged are not reused.
				However, if less than 'maxPurgeCacheDelay' time have passed since the
				previous request to taskcluster-purge-cache then the request is skipped.

				This defaults to 3 minutes, which is reasonable in most cases.
			`),
		},
		"purgeCacheBaseUrl": schematypes.URI{
			Title: "BaseUrl for purge-cache service",
			Description: util.Markdown(`
				This is the baseUrl for the taskcluster-purge-cache service, which tells
				what caches should be purged.

				This defaults to the production value from taskcluster-client libraries.
				You do not need to set this in production.
			`),
		},
	},
}
