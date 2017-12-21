#!/bin/bash

touch .bash_history
CGO_ENABLED=${CGO_ENABLED:-1}
ARGS="--tty --interactive --rm --privileged -e DEBUG -e GOARCH -e CGO_ENABLED=$CGO_ENABLED";
ARGS="$ARGS -e TASKCLUSTER_CLIENT_ID -e TASKCLUSTER_ACCESS_TOKEN -e TASKCLUSTER_CERTIFICATE";
ARGS="$ARGS -e PULSE_USERNAME -e PULSE_PASSWORD";
ARGS="$ARGS -v `pwd`/.bash_history:/root/.bash_history";
ARGS="$ARGS -v `pwd`:/go/src/github.com/taskcluster/taskcluster-worker/";
ARGS="$ARGS taskcluster/tc-worker-env";

TAGS='qemu network system native';

if [[ "$@" == go\ * ]]; then
  docker run $ARGS "$@";
elif [[ "$1" == -- ]]; then
  shift;
  docker run $ARGS "$@";
elif [[ "$@" == bash ]]; then
  docker run $ARGS bash --login;
elif [[ "$@" == "" ]]; then
  docker run $ARGS go test -race -tags "$TAGS" -p 1 -v \
  `go list ./... | grep -v ^github.com/taskcluster/taskcluster-worker/vendor/`;
else
  docker run $ARGS go test -v -race -tags "$TAGS" -p 1 "$@";
fi;

if [[ "$?" != "0" ]]; then
  echo "### TEST FAILED";
  exit 1;
else
  echo "### TEST PASSED";
fi
