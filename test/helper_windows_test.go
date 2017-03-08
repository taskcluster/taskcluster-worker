package test

import "strconv"

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
