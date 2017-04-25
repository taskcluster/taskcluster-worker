// +build linux,native darwin,native

package nativetest

import (
	"testing"

	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/worker/workertest"
)

/*
 - folder with multiple artifacts, and sub-folders
 - content type for folder artifacts
 - file artifact is a folder
*/

func TestArtifacts(t *testing.T) {
	debug("### Testing artifact plugin with native engine")
	debug("generated filename: %s", filename)
	artifactCase.Test(t)
}

var filename = slugid.Nice()
var artifactCase = workertest.Case{
	Engine:       "native",
	Concurrency:  1,
	EngineConfig: engineConfig,
	PluginConfig: pluginConfig,
	Tasks: []workertest.Task{
		{
			Title:   "Artifact File",
			Success: true,
			Payload: `{
				"command": ["sh", "-c", "echo 'hello-world' && echo 42 > ` + filename + `.txt"],
				"env": {},
				"maxRunTime": "10 minutes",
				"artifacts": [
					{
						"name": "public/result.txt",
						"type": "file",
						"path": "` + filename + `.txt"
					}
				]
			}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
				"public/result.txt":            workertest.GrepArtifact("42"),
			},
		},
		{
			Title:   "Artifact Directory",
			Success: true,
			Payload: `{
				"command": ["sh", "-c", "echo 'hello-world' && mkdir -p sub/subsub/ && echo 42 > sub/subsub/result.txt"],
				"env": {},
				"maxRunTime": "10 minutes",
				"artifacts": [
					{
						"name": "public",
						"type": "directory",
						"path": "sub"
					}
				]
			}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("hello-world"),
				"public/subsub/result.txt":     workertest.GrepArtifact("42"),
			},
		},
		{
			Title:   "Artifact Directory Is File",
			Success: false,
			Payload: `{
				"command": ["sh", "-c", "echo 42 > notafolder"],
				"env": {},
				"maxRunTime": "10 minutes",
				"artifacts": [
					{
						"name": "public/myfolder",
						"type": "directory",
						"path": "notafolder"
					}
				]
			}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("notafolder"),
				"public/myfolder":              workertest.ErrorArtifact(),
			},
		},
		{
			Title:   "Artifact File Not Found",
			Success: false,
			Payload: `{
				"command": ["true"],
				"env": {},
				"maxRunTime": "10 minutes",
				"artifacts": [
					{
						"name": "public/result.txt",
						"type": "file",
						"path": "no-such-file-` + filename + `.txt"
					}
				]
			}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("no-such-file-" + filename + ".txt"),
				"public/result.txt":            workertest.ErrorArtifact(),
			},
		},
		{
			Title:   "Artifact Directory Not Found",
			Success: false,
			Payload: `{
				"command": ["sh", "-c", "true"],
				"env": {},
				"maxRunTime": "10 minutes",
				"artifacts": [
					{
						"name": "public/myfolder",
						"type": "directory",
						"path": "no-such-folder/no-sub-folder"
					}
				]
			}`,
			Artifacts: workertest.ArtifactAssertions{
				"public/logs/live.log":         workertest.ReferenceArtifact(),
				"public/logs/live_backing.log": workertest.GrepArtifact("no-such-folder/no-sub-folder"),
				"public/myfolder":              workertest.ErrorArtifact(),
			},
		},
		// NOTE: If anyone can come up with an artifact path is illegal please add a test case
	},
}
