package osxnative

import (
	"os"
	"os/exec"
	"strings"
)

type dscl struct {
}

// run will exectue a dscl command using the provided arguments
// returning the output of the command as a string.
func (d dscl) run(command string, a ...string) (string, error) {
	args := []string{".", command}
	args = append(args, a...)
	cmd := exec.Command("dscl", args...)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	return string(output), err
}

func (d dscl) list(a ...string) ([][]string, error) {
	output, err := d.run("list", a...)

	if err != nil {
		return nil, err
	}

	ret := [][]string{}
	for _, s := range strings.Split(output, "\n") {
		fields := strings.Fields(s)
		if len(fields) != 0 {
			ret = append(ret, strings.Fields(s))
		}
	}

	return ret, nil
}

func (d dscl) read(a ...string) (string, error) {
	s, err := d.run("read", a...)

	if err != nil {
		return s, err
	}

	return strings.Fields(s)[1], nil
}

func (d dscl) create(a ...string) error {
	_, err := d.run("create", a...)
	return err
}

func (d dscl) append(a ...string) error {
	_, err := d.run("append", a...)
	return err
}

func (d dscl) delete(a ...string) error {
	_, err := d.run("delete", a...)
	return err
}
