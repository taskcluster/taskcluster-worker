// +build darwin

package osxnative

import (
	"os"
	osuser "os/user"

	"github.com/taskcluster/taskcluster-worker/runtime"
)

// Get the child process working.
// If we could create a new user, the working is the user home
// directory, otherwise it is the parent's working dir.
func getWorkingDir(u user, context *runtime.TaskContext) string {
	var cwd string
	var err error

	// In case of error we panic here because error at this point means
	// something is terribly wrong
	if u.name != "" {
		userInfo, err2 := osuser.Lookup(u.name)
		if err2 != nil {
			context.LogError("user.Lookup failed: ", err2, "\n")
			panic(err2)
		}
		cwd = userInfo.HomeDir
	} else {
		cwd, err = os.Getwd()
		if err != nil {
			context.LogError("Getwd failed: ", err, "\n")
			panic(err)
		}
	}

	return cwd
}
