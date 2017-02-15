package integrationtest

import (
	"path/filepath"

	"github.com/taskcluster/taskcluster-worker/commands"
)

func RunTestWorker() {
	commands.Run(
		[]string{
			"work",
			filepath.Join("testdata", "worker-config.yml"),
		},
	)
}
