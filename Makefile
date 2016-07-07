# A minimalistic makefile for common tasks.
# We're not supposed to use makefiles with go, but sometimes it's hard to
# remember the right command. So let's keep this simple, just targets and
# commands.

# Ensure go-extpoints and go-import-subtree are available for go generate
export PATH := $(GOPATH)/bin:$(PATH)

all: build
build:
	go fmt $$(go list ./... | grep -v /vendor/)
	go install $$(go list ./... | grep -v /vendor/)

generate:
	# these three tools are needed for go generate steps later...
	go install github.com/taskcluster/taskcluster-worker/vendor/github.com/progrium/go-extpoints
	go install github.com/taskcluster/taskcluster-worker/vendor/github.com/jonasfj/go-import-subtree
	go install github.com/taskcluster/taskcluster-worker/codegen/go-composite-schema
	# now we have the code generation tools built, we can use them...
	# note, we can't use go generate ./... as we'll pick up vendor packages and have problems, so
	# we use an explicit list
	go generate $$(go list ./... | grep -v /vendor/)
	go fmt $$(go list ./... | grep -v /vendor/)

rebuild: generate build test

check: test
	# tests should fail if go generate or go fmt results in uncommitted code
	git status --porcelain
	/bin/bash -c 'test $$(git status --porcelain | wc -l) == 0'
test:
	go install github.com/taskcluster/taskcluster-worker/vendor/github.com/golang/lint/golint
	go test -v -race $$(go list ./... | grep -v /vendor/)
	go vet $$(go list ./... | grep -v /vendor/)
	go list ./... | grep -v /vendor/ | xargs -n1 golint

dev-test:
	go test -race $$(go list ./... | grep -v /vendor/)
