package system

import "fmt"

// Utilies that are useful across platforms

func formatEnv(env map[string]string) []string {
	environ := []string{}
	for k, v := range env {
		environ = append(environ, fmt.Sprintf("%s=%s", k, v))
	}
	return environ
}

func formatArgs(options map[string]string) []string {
	args := []string{}
	for option, arg := range options {
		args = append(args, option, arg)
	}
	return args
}
