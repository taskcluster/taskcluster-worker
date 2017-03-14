package test

import (
	"path/filepath"
	"strconv"
	"strings"
)

func helloGoodbye() []string {
	return []string{
		"cmd.exe",
		"/c",
		"echo hello world! && echo goodbye world!",
	}
}

func failCommand() []string {
	return []string{
		"exit",
		"1",
	}
}

func sleep(seconds uint) []string {
	return []string{
		"cmd.exe",
		"/c",
		"ping 127.0.0.1 -n " + strconv.Itoa(int(seconds+1)) + " > nul",
	}
}

func copyArtifact(path string) []string {
	targetFile := strings.Replace(path, "/", "\\", -1)
	sourceFile := filepath.Join(testdataDir, targetFile)
	return []string{
		"cmd.exe",
		"/c",
		"echo \"copying file(s)\"" +
			" && rmdir \"" + targetFile + "\" /s /q" +
			" && mkdir \"" + filepath.Dir(targetFile) + "\"" +
			" && xcopy \"" + sourceFile + "\" \"" + targetFile + "\" /e /i /f /h /k /o /x /y",
	}
}

func resolveTask() []string {
	return []string{
		"go",
		"run",
		filepath.Join(testdata, "..", "resolvetask", "resolvetask.go"),
	}
}
