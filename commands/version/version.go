package version

import (
	"regexp"
	"strings"
)

var tags = ""     // -ldflags "-X main.version '`git tag -l --points-at HEAD`'"
var revision = "" // -ldflags "-X main.version '`git rev-parse HEAD`'"

var versionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// Version returns the semver version of this taskcluster-worker build on the
// form 0.0.0, or empty string if not built with a version number.
func Version() string {
	for _, tag := range strings.Split(tags, "\n") {
		if versionPattern.MatchString(tag) {
			return tag[1:]
		}
	}
	return ""
}

// Revision returns the git revision hash of this taskcluster-worker build, or
// empty string if not injected at build-time.
func Revision() string {
	if len(revision) != 40 {
		return ""
	}
	return revision
}
