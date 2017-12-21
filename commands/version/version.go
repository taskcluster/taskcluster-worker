package version

import "regexp"

var version = ""  // -ldflags "-X github.com/taskcluster/taskcluster-worker/commands/version.version=`git tag -l 'v*.*.*' --points-at HEAD | head -n1`"
var revision = "" // -ldflags "-X github.com/taskcluster/taskcluster-worker/commands/version.revision=`git rev-parse HEAD`"

func init() {
	debug("version: '%s' from linker flag", version)
	// Version pattern for sanity checks, so we know if this was built with a valid
	// version number or not.
	versionPattern := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	if versionPattern.MatchString(version) {
		version = version[1:] // remove the 'v' prefix from the tag
	} else {
		version = ""
	}

	debug("revision: '%s' from linker flag", revision)
	// Sanity check for git revision hash
	if len(revision) != 40 {
		revision = ""
	}

	debug("detected version: '%s', revision: '%s'", version, revision)
}

// Version returns the semver version of this taskcluster-worker build on the
// form 0.0.0, or empty string if not built with a version number.
func Version() string {
	return version
}

// Revision returns the git revision hash of this taskcluster-worker build, or
// empty string if not injected at build-time.
func Revision() string {
	return revision
}
