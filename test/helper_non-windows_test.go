// +build !windows

package test

import "strconv"

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
