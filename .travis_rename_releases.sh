#!/bin/bash

# The deploy is called per arch and os combination - so we only release one file here.
# We just need to work out which file we built, rename it to something unique, and
# set an environment variable for its location that we can use in .travis.yml for
# publishing back to github.

# linux, darwin:
FILE_EXT=""
[ "${GOOS}" == "windows" ] && FILE_EXT=".exe"

# let's rename the release file because it has a 1:1 mapping with what it is called on
# github releases, and therefore the name for each platform needs to be unique so that
# they don't overwrite each other. Set a variable that can be used in .travis.yml
export RELEASE_FILE="${TRAVIS_BUILD_DIR}/taskcluster-worker-${TRAVIS_TAG:1}-${GOOS}-${GOARCH}${FILE_EXT}"
mv "${GOPATH}/bin/${GOOS}_${GOARCH}/taskcluster-worker${FILE_EXT}" "${RELEASE_FILE}"
