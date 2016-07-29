#!/bin/bash -e

ARGS="--tty --interactive --rm --privileged -e DEBUG -v `pwd`:/src taskcluster/tc-worker-env"

if [[ "$@" == go\ * ]]; then
  exec docker run $ARGS $@;
elif [[ "$@" == bash ]]; then
  exec docker run $ARGS bash --login;
elif [[ "$@" == goconvey ]]; then
  exec docker run -p 8080:8080 $ARGS goconvey -packages 1 -launchBrowser=false --host 0.0.0.0 -port 8080;
elif [[ "$@" == "" ]]; then
  exec docker run $ARGS go test -race -tags qemu -v \
  `go list ./... | grep -v ^github.com/taskcluster/taskcluster-worker/vendor/`;
else
  exec docker run $ARGS go test -v -race -tags qemu $@;
fi
