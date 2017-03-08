package test

func helloGoodbye() []string {
	return []string{
		"cmd.exe",
		"/c",
		"echo hello world! && echo goodbye world!",
	}
}

func failCommand() []string {
	return []string{
		"exit 1",
	}
}
