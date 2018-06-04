// +build linux,docker

package dockertest

import (
	"strings"
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

func TestArtifacts(t *testing.T) {
	debug("### Testing artifact plugin with docker engine")
	filename := slugid.Nice()
	debug("generated filename: %s", filename)

	workertest.Case{
		Engine:       "docker",
		Concurrency:  1,
		EngineConfig: engineConfig,
		PluginConfig: pluginConfig,
		Tasks: workertest.Tasks([]workertest.Task{{
			Title:   "Artifact File",
			Success: true,
			Payload: `{
						"image": "` + dockerImageName + `",
						"command": ["sh", "-c", "echo 'hello-world' && echo 42 > /my-file-` + filename + `.txt"],
						"env": {},
						"maxRunTime": "10 minutes",
						"artifacts": [
							{
								"name": "public/result.txt",
								"type": "file",
								"path": "/my-file-` + filename + `.txt"
							}
						]
					}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
				"public/result.txt":            workertest.GrepArtifact("42"),
			},
		}, {
			Title:   "Artifact Directory",
			Success: true,
			Payload: `{
					"image": "` + dockerImageName + `",
					"command": ["sh", "-c", "` + strings.Join([]string{
				"echo 'hello-world'",
				"mkdir -p /etc/test",
				"echo 'hello-world' > /etc/test/my-file.txt",
			}, " && ") + `"],
					"env": {},
					"maxRunTime": "10 minutes",
					"artifacts": [
						{
							"name": "public/test",
							"type": "directory",
							"path": "/etc/test"
						}
					]
				}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
				"public/test/my-file.txt":      workertest.GrepArtifact("hello-world"),
			},
		}, {
			Title:   "Nested Artifact Directory",
			Success: true,
			Payload: `{
						"image": "` + dockerImageName + `",
						"command": ["sh", "-c", "` + strings.Join([]string{
				"echo 'hello-world'",
				"mkdir -p /sub/subsub/subsubsub",
				"echo 42 > /sub/subsub/result.txt",
				"echo -n '<html></html>' > /sub/subsub/subsubsub/result.html",
			}, " && ") + `"],
						"env": {},
						"maxRunTime": "10 minutes",
						"artifacts": [
							{
								"name": "public",
								"type": "directory",
								"path": "/sub"
							}
						]
					}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":                workertest.ReferenceArtifact(),
				"public/logs/live_backing.log":        workertest.GrepArtifact("hello-world"),
				"public/subsub/result.txt":            workertest.GrepArtifact("42"),
				"public/subsub/subsubsub/result.html": workertest.MatchArtifact("<html></html>", "text/html; charset=utf-8"),
			},
		}, {
			Title:   "Artifact Directory Is File",
			Success: false,
			Payload: `{
					"image": "` + dockerImageName + `",
					"command": ["sh", "-c", "echo 42 > /notafolder"],
					"env": {},
					"maxRunTime": "10 minutes",
					"artifacts": [
						{
							"name": "public/myfolder",
							"type": "directory",
							"path": "/notafolder"
						}
					]
				}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("/notafolder"),
				"public/myfolder":              workertest.ErrorArtifact(),
			},
		}, {
			Title:   "Artifact File Is Directory",
			Success: false,
			Payload: `{
						"image": "` + dockerImageName + `",
						"command": ["sh", "-c", "mkdir -p /sub/subsub/ && echo 42 > /sub/subsub/result.txt"],
						"env": {},
						"maxRunTime": "10 minutes",
						"artifacts": [
							{
								"name": "public/subsub",
								"type": "file",
								"path": "/sub/subsub"
							}
						]
					}`,
			Artifacts: workertest.ArtifactAssertions{
				// Expect some error message saying "sub/subsub"
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("/sub/subsub"),
				"public/subsub":                workertest.ErrorArtifact(),
			},
		}, {
			Title:   "Artifact File Not Found",
			Success: false,
			Payload: `{
						"image": "` + dockerImageName + `",
						"command": ["true"],
						"env": {},
						"maxRunTime": "10 minutes",
						"artifacts": [
							{
								"name": "public/result.txt",
								"type": "file",
								"path": "/no-such-file-` + filename + `.txt"
							}
						]
					}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("/no-such-file-" + filename + ".txt"),
				"public/result.txt":            workertest.ErrorArtifact(),
			},
		}, {
			Title:   "Artifact Directory Not Found",
			Success: false,
			Payload: `{
						"image": "` + dockerImageName + `",
						"command": ["sh", "-c", "true"],
						"env": {},
						"maxRunTime": "10 minutes",
						"artifacts": [
							{
								"name": "public/myfolder",
								"type": "directory",
								"path": "/no-such-folder/no-sub-folder"
							}
						]
					}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("/no-such-folder/no-sub-folder"),
				"public/myfolder":              workertest.ErrorArtifact(),
			},
			// NOTE: If anyone can come up with an artifact path is illegal please add a test case
		}}),
	}.Test(t)
}
