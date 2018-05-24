// +build linux,docker

package dockertest

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestEmptyCache(t *testing.T) {
	workertest.Case{
		Engine:       "docker",
		Concurrency:  0, // run in order given, by adding dependency links
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{{
			Title:   "Write Empty Cache",
			Scopes:  []string{"worker:cache:dummy-cache-from-task-1"}, // caches with a name requires a scope
			Success: true,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "echo 'hello-world-task-1' > /mnt/cache-folder/cache-file.txt"],
				"env": {},
				"maxRunTime": "10 minutes",
				"caches": [
					{
						"name": "dummy-cache-from-task-1",
						"mountPoint": "/mnt/cache-folder/",
						"options": {}
					}
				]
			}`,
			AllowAdditional: true,
		}, {
			Title:   "Read From Cache",
			Scopes:  []string{"worker:cache:dummy-cache-from-task-1"},
			Success: true,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "cat /mnt/from-task-1/cache-file.txt"],
				"env": {},
				"maxRunTime": "10 minutes",
				"caches": [
					{
						"name": "dummy-cache-from-task-1",
						"mountPoint": "/mnt/from-task-1/",
						"options": {}
					}
				]
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world-task-1"),
			},
		}}),
	}.Test(t)
}

func TestPreloadedCache(t *testing.T) {
	var m sync.Mutex
	count := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		debug("-> %s %s", r.Method, r.URL.Path)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		debug("<- tar-stream")
		m.Lock()
		defer m.Unlock()
		count++
		w.WriteHeader(http.StatusOK)
		err := writeTarStream(w, []tarEntry{{
			Name:   "info.txt",
			IsFile: true,
			Data:   "hello-from-info.txt",
		}, {
			Name:   "data/",
			IsFile: false,
			Data:   "",
		}, {
			Name:   "data/loaded-count",
			IsFile: true,
			Data:   fmt.Sprintf("loaded: %d", count),
		}})
		assert.NoError(t, err, "failed to write tar-stream")
	}))
	defer s.Close()

	workertest.Case{
		Engine:       "docker",
		Concurrency:  0, // run in order given, by adding dependency links
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{{
			Title:   "Preloaded Read-Only Cache", // caches without a "name" are read-only
			Success: true,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "cat /mnt/preloaded-cache/info.txt && cat /mnt/preloaded-cache/data/loaded-count"],
				"env": {},
				"maxRunTime": "10 minutes",
				"caches": [
					{
						"mountPoint": "/mnt/preloaded-cache/",
						"options": {},
						"preload": "` + s.URL + `"
					}
				]
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.AllOfArtifact(
					workertest.GrepArtifact("hello-from-info.txt"),
					workertest.GrepArtifact("loaded: 1"),
				),
			},
		}, {
			Title:   "Reuse Preloaded Read-Only Cache",
			Success: true,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "cat /mnt/preloaded-cache/data/loaded-count"],
				"env": {},
				"maxRunTime": "10 minutes",
				"caches": [
					{
						"mountPoint": "/mnt/preloaded-cache/",
						"options": {},
						"preload": "` + s.URL + `"
					}
				]
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.GrepArtifact("loaded: 1"),
			},
		}, {
			Title:   "Reload Preloaded Read-Only Cache",
			Success: true,
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "cat /mnt/preloaded-cache/data/loaded-count"],
				"env": {},
				"maxRunTime": "10 minutes",
				"caches": [
					{
						"mountPoint": "/mnt/preloaded-cache/",
						"options": {},
						"preload": "` + s.URL + `/other-path-triggering-a-reload"
					}
				]
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.GrepArtifact("loaded: 2"),
			},
		}, {
			Title:   "Preloaded Read-Write Cache",
			Success: true,
			Scopes:  []string{"worker:cache:dummy-cache-from-task-preloaded"},
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "` + strings.Join([]string{
				"cat /mnt/preloaded-cache/info.txt",
				"cat /mnt/preloaded-cache/data/loaded-count",
				"echo 'cached-value' > /mnt/preloaded-cache/my-new-file.txt", // write value to next task
			}, " && ") + `"],
				"env": {},
				"maxRunTime": "10 minutes",
				"caches": [
					{
						"name": "dummy-cache-from-task-preloaded",
						"mountPoint": "/mnt/preloaded-cache/",
						"options": {},
						"preload": "` + s.URL + `"
					}
				]
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.AllOfArtifact(
					workertest.GrepArtifact("hello-from-info.txt"),
					workertest.GrepArtifact("loaded: 3"),
				),
			},
		}, {
			Title:   "Reuse Read-Write Cache",
			Success: true,
			Scopes:  []string{"worker:cache:dummy-cache-from-task-preloaded"},
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "` + strings.Join([]string{
				"cat /mnt/preloaded-cache/info.txt",
				"cat /mnt/preloaded-cache/data/loaded-count",
				"cat /mnt/preloaded-cache/my-new-file.txt", // read value written last time
			}, " && ") + `"],
				"env": {},
				"maxRunTime": "10 minutes",
				"caches": [
					{
						"name": "dummy-cache-from-task-preloaded",
						"mountPoint": "/mnt/preloaded-cache/",
						"options": {},
						"preload": "` + s.URL + `"
					}
				]
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.AllOfArtifact(
					workertest.GrepArtifact("hello-from-info.txt"),
					workertest.GrepArtifact("loaded: 3"),
					workertest.GrepArtifact("cached-value"), // written last time
				),
			},
		}, {
			Title:     "Missing Cache Scope",
			Success:   false,
			Exception: runtime.ReasonMalformedPayload,
			Scopes:    []string{"worker:cache:dummy-cache-wrong-cache"},
			Payload: `{
				"image": "` + dockerImageName + `",
				"command": ["sh", "-c", "` + strings.Join([]string{
				"cat /mnt/preloaded-cache/info.txt",
				"cat /mnt/preloaded-cache/data/loaded-count",
				"cat /mnt/preloaded-cache/my-new-file.txt",
			}, " && ") + `"],
				"env": {},
				"maxRunTime": "10 minutes",
				"caches": [
					{
						"name": "dummy-cache-from-task-preloaded",
						"mountPoint": "/mnt/preloaded-cache/",
						"options": {},
						"preload": "` + s.URL + `"
					}
				]
			}`,
			AllowAdditional: true,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live_backing.log": workertest.AllOfArtifact(
					// Should show the scope that is missing
					workertest.GrepArtifact("worker:cache:dummy-cache-from-task-preloaded"),
					// Inverted assertions for good measure
					workertest.NotGrepArtifact("hello-from-info.txt"),
					workertest.NotGrepArtifact("loaded: 3"),
					workertest.NotGrepArtifact("cached-value"),
				),
			},
		}}),
	}.Test(t)
}

type tarEntry struct {
	Name   string
	IsFile bool
	Data   string
}

// writeTarStream writes entries to w as a tar-stream
func writeTarStream(w io.Writer, entries []tarEntry) error {
	tw := tar.NewWriter(w)

	// Write all entries
	for _, entry := range entries {
		debug(" * '%s'", entry.Name)

		// Find data and typeflag
		var data []byte
		var kind byte
		if entry.IsFile {
			kind = tar.TypeReg
			data = []byte(entry.Data)
		} else {
			kind = tar.TypeDir
			data = []byte{}
		}

		// Write entry
		err := tw.WriteHeader(&tar.Header{
			Name:     entry.Name,
			Mode:     0777,
			Typeflag: kind,
			Size:     int64(len(data)),
		})
		if err != nil {
			return err
		}
		_, err = tw.Write(data)
		if err != nil {
			return err
		}
	}
	return tw.Close()
}
