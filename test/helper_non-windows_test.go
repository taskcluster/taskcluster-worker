// +build !windows

package test

import (
	"path/filepath"
	"strconv"
)

func helloGoodbye() []string {
	return []string{
		"echo",
		"hello world!\ngoodbye world!",
	}
}

func sleep(seconds uint) []string {
	return []string{
		"sleep",
		strconv.Itoa(int(seconds)),
	}
}

func failCommand() []string {
	return []string{
		"false",
	}
}

func copyArtifact(path string) []string {
	sourcePath := filepath.Join(testdata, path)
	return []string{
		"/bin/bash",
		"-exvc",
		"echo \"copying file(s)\"\n" +
			"rm -rf '" + path + "'\n" +
			"mkdir -p '" + filepath.Dir(path) + "'\n" +
			"cp -pr '" + sourcePath + "' '" + path + "'",
	}
}
