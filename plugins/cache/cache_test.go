package cache

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/worker/workertest"

	_ "github.com/taskcluster/taskcluster-worker/engines/mock"
	_ "github.com/taskcluster/taskcluster-worker/plugins/livelog"
	_ "github.com/taskcluster/taskcluster-worker/plugins/success"
)

const testPluginConfig = `{
	"disabled": [],
	"success": {},
	"livelog": {},
	"cache": {}
}`

func TestReadWriteEmptyCache(t *testing.T) {
	workertest.Case{
		Concurrency:  0, // runs tasks sequentially
		Engine:       "mock",
		EngineConfig: `{}`,
		PluginConfig: testPluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{
			{
				Title:  "Write hello-world to empty cache volume",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "write-volume",
					"argument": "my-mount-point/my-folder/my-file.txt:hello-world",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "my-mount-point",
							"options": {}
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.AnyArtifact(),
				},
				AllowAdditional: true,
				Success:         true,
			}, {
				Title:  "Read from cache volume",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "read-volume",
					"argument": "some-mount-point/my-folder/my-file.txt",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "some-mount-point",
							"options": {}
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
				},
				AllowAdditional: true,
				Success:         true,
			},
		}),
	}.TestWithFakeQueue(t) // TODO: Resolve scope issues and test against real queue
}

func TestReadPreloadCache(t *testing.T) {
	// Create a tiny tar archive in-memory
	buf := bytes.NewBuffer(nil)
	text := []byte("hej verden")
	a := tar.NewWriter(buf)
	err := a.WriteHeader(&tar.Header{
		Name: "min-mappe/min-fil.txt",
		Mode: 0777,
		Size: int64(len(text)),
	})
	require.NoError(t, err, "failed to create file header in tar archive")
	_, err = a.Write(text)
	require.NoError(t, err, "failed to write file body in tar archive")
	err = a.Close()
	require.NoError(t, err, "failed to create tar archive")
	rawtar := buf.Bytes()

	// Create a test server that serves a tar archive
	var m sync.Mutex
	count := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.Lock()
		count++
		m.Unlock()
		w.Header().Set("Content-Type", "application/tar")
		w.Header().Set("Content-Length", strconv.Itoa(len(rawtar)))
		w.WriteHeader(http.StatusOK)
		w.Write(rawtar)
	}))
	defer s.Close()
	defer func() {
		m.Lock()
		require.Equal(t, 2, count, "expected exactly 2 fetches")
		m.Unlock()
	}()

	workertest.Case{
		Concurrency:  0, // runs tasks sequentially
		Engine:       "mock",
		EngineConfig: `{}`,
		PluginConfig: testPluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{
			{
				Title:  "Read from cache volume",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "read-volume",
					"argument": "my-mount-point/min-mappe/min-fil.txt",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "my-mount-point",
							"options": {},
							"preload": "` + s.URL + `"
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.GrepArtifact("hej verden"),
				},
				AllowAdditional: true,
				Success:         true,
			}, {
				Title:  "Read from cache volume again",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "read-volume",
					"argument": "my-mount-point/min-mappe/min-fil.txt",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "my-mount-point",
							"options": {},
							"preload": "` + s.URL + `"
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.GrepArtifact("hej verden"),
				},
				AllowAdditional: true,
				Success:         true,
			}, {
				Title:  "Write to preloaded cache volume",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "write-volume",
					"argument": "my-mount-point/min-mappe/min-fil.txt:hello-world",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "my-mount-point",
							"options": {},
							"preload": "` + s.URL + `"
						}
					]
				}`,
				AllowAdditional: true,
				Success:         true,
			}, {
				Title:  "Read from cache volume after write",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "read-volume",
					"argument": "my-mount-point/min-mappe/min-fil.txt",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "my-mount-point",
							"options": {},
							"preload": "` + s.URL + `"
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
				},
				AllowAdditional: true,
				Success:         true,
			}, {
				Title: "Read from read-only cache volume",
				Payload: `{
					"delay": 5,
					"function": "read-volume",
					"argument": "my-mount-point/min-mappe/min-fil.txt",
					"caches": [
						{
							"mountPoint": "my-mount-point",
							"options": {},
							"preload": "` + s.URL + `"
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.GrepArtifact("hej verden"),
				},
				AllowAdditional: true,
				Success:         true,
			}, {
				Title: "Read from read-only cache volume again",
				Payload: `{
					"delay": 5,
					"function": "read-volume",
					"argument": "my-other-mount-point/min-mappe/min-fil.txt",
					"caches": [
						{
							"mountPoint": "my-other-mount-point",
							"options": {},
							"preload": "` + s.URL + `"
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.GrepArtifact("hej verden"),
				},
				AllowAdditional: true,
				Success:         true,
			},
			{
				Title: "Write to preloaded read-only cache volume fails",
				Payload: `{
					"delay": 5,
					"function": "write-volume",
					"argument": "my-mount-point/min-mappe/min-fil.txt:hello-world",
					"caches": [
						{
							"mountPoint": "my-mount-point",
							"options": {},
							"preload": "` + s.URL + `"
						}
					]
				}`,
				AllowAdditional: true,
				Success:         false,
			},
		}),
	}.TestWithFakeQueue(t) // TODO: Resolve scope issues and test against real queue
}

func TestCacheScopeRequired(t *testing.T) {
	workertest.Case{
		Concurrency:  0, // runs tasks sequentially
		Engine:       "mock",
		EngineConfig: `{}`,
		PluginConfig: testPluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{
			{
				Title:  "Write hello-world to empty cache volume",
				Scopes: []string{"worker:cache:dummy-garbage-wrong-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "write-volume",
					"argument": "my-mount-point/my-folder/my-file.txt:hello-world",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "my-mount-point",
							"options": {}
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.GrepArtifact("dummy-garbage-my-cache-name"),
				},
				AllowAdditional: true,
				Exception:       runtime.ReasonMalformedPayload,
				Success:         false,
			}, {
				Title:  "Access with star-scope",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-*"},
				Payload: `{
					"delay": 5,
					"function": "write-volume",
					"argument": "my-mount-point/my-folder/my-file.txt:hello-world",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "my-mount-point",
							"options": {}
						}
					]
				}`,
				AllowAdditional: true,
				Success:         true,
			},
		}),
	}.TestWithFakeQueue(t) // TODO: Resolve scope issues and test against real queue
}

func TestPurgeCache(t *testing.T) {
	prevDefaultMaxPurgeCacheDelay := defaultMaxPurgeCacheDelay
	defaultMaxPurgeCacheDelay = 0
	defer func() {
		defaultMaxPurgeCacheDelay = prevDefaultMaxPurgeCacheDelay
	}()

	var purging atomics.Bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start-purging" {
			purging.Set(true)
			w.WriteHeader(http.StatusOK)
			w.Write(nil)
			return
		}

		parts := strings.Split(r.URL.Path, "/")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		var requests interface{}
		if purging.Get() {
			requests = []interface{}{
				map[string]interface{}{
					"provisionerId": parts[2],
					"workerType":    parts[3],
					"cacheName":     "dummy-garbage-my-cache-name",
					"before":        time.Now().UTC(),
				},
			}
		} else {
			requests = []interface{}{
				map[string]interface{}{
					"provisionerId": parts[2],
					"workerType":    parts[3],
					"cacheName":     "dummy-garbage-wrong-name",
					"before":        time.Now().UTC(),
				},
			}
		}
		data, _ := json.Marshal(map[string]interface{}{
			"requests": requests,
		})
		w.Write(data)
	}))
	defer s.Close()

	workertest.Case{
		Concurrency:  0, // runs tasks sequentially
		Engine:       "mock",
		EngineConfig: `{}`,
		PluginConfig: `{
			"disabled": [],
			"success": {},
			"livelog": {},
			"cache": {
				"purgeCacheBaseUrl": "` + s.URL + `"
			}
		}`,
		Tasks: workertest.Tasks([]workertest.Task{
			{
				Title:  "Write hello-world to empty cache volume",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "write-volume",
					"argument": "my-mount-point/my-folder/my-file.txt:hello-world",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "my-mount-point",
							"options": {}
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.AnyArtifact(),
				},
				AllowAdditional: true,
				Success:         true,
			}, {
				Title:  "Read from cache volume",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "read-volume",
					"argument": "some-mount-point/my-folder/my-file.txt",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "some-mount-point",
							"options": {}
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
				},
				AllowAdditional: true,
				Success:         true,
			}, {
				Title: "Ping purge-cache service to start purging",
				Payload: `{
					"delay": 5,
					"function": "get-url",
					"argument": "` + s.URL + `/start-purging"
				}`,
				AllowAdditional: true,
				Success:         true,
			}, {
				Title:  "Read from cache volume after purge",
				Scopes: []string{"worker:cache:dummy-garbage-my-cache-name"},
				Payload: `{
					"delay": 5,
					"function": "read-volume",
					"argument": "some-mount-point/my-folder/my-file.txt",
					"caches": [
						{
							"name": "dummy-garbage-my-cache-name",
							"mountPoint": "some-mount-point",
							"options": {}
						}
					]
				}`,
				Artifacts: workertest.ArtifactAssertions{
					"public/logs/live_backing.log": workertest.NotGrepArtifact("hello-world"),
				},
				AllowAdditional: true,
				Success:         false,
			},
		}),
	}.TestWithFakeQueue(t) // TODO: Resolve scope issues and test against real queue
}
