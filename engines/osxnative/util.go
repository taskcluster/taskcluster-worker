package osxnative

import (
	"github.com/taskcluster/taskcluster-worker/runtime"
	"os"
	osuser "os/user"
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
		userInfo, err := osuser.Lookup(u.name)
		if err != nil {
			context.LogError("user.Lookup failed: ", err, "\n")
			panic(err)
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
