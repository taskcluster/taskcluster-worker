#!/bin/bash

touch .bash_history
ARGS="--tty --interactive --rm --privileged -e DEBUG";
ARGS="$ARGS -v `pwd`/.bash_history:/root/.bash_history";
ARGS="$ARGS -v `pwd`:/go/src/github.com/taskcluster/taskcluster-worker/";
ARGS="$ARGS taskcluster/tc-worker-env";

TAGS='qemu network system native';

if [[ "$@" == go\ * ]]; then
  docker run $ARGS $@;
elif [[ "$1" == -- ]]; then
  shift;
  docker run $ARGS $@;
elif [[ "$@" == bash ]]; then
  docker run $ARGS bash --login;
elif [[ "$@" == "" ]]; then
  docker run $ARGS go test -race -tags "$TAGS" -p 1 -v \
  `go list ./... | grep -v ^github.com/taskcluster/taskcluster-worker/vendor/`;
else
  docker run $ARGS go test -v -race -tags "$TAGS" -p 1 $@;
fi;

if [[ "$?" != "0" ]]; then
  echo "### TEST FAILED";
  exit 1;
else
  echo "### TEST PASSED";
fi
