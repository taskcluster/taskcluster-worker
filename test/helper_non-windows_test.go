// +build !windows

package test

func helloGoodbye() []string {
	return []string{
		"echo",
		"hello world!\ngoodbye world!",
	}
}
