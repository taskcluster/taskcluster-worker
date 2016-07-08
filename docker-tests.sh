#!/bin/bash -e

docker run -ti --rm --privileged -e DEBUG -v `pwd`:/src taskcluster/tc-worker-env \
  go test -race -tags qemu "$@"
