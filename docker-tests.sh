#!/bin/bash -e

docker run -ti --rm --privileged -v `pwd`:/src taskcluster/tc-worker-env \
  go test -race -tags qemu "$@"
