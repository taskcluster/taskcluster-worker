# A minimalistic makefile for common tasks.
# We're not supposed to use makefiles with go, but sometimes it's hard to
# remember the right command. So let's keep this simple, just targets and
# commands.

# Ensure go-extpoints and go-import-subtree are available for go generate
export PATH := $(GOPATH)/bin:$(PATH)

all: build
build:
	go fmt ./...
	go get ./...

generate:
	go get github.com/progrium/go-extpoints
	go get github.com/jonasfj/go-import-subtree
	go get github.com/taskcluster/taskcluster-worker/codegen/...
	go get github.com/rjeczalik/interfaces/cmd/interfacer
	go get github.com/vektra/mockery/...
	go generate ./...
	go fmt ./...

rebuild: generate build test

check: test
	# tests should fail if go generate or go fmt results in uncommitted code
	git status --porcelain
	/bin/bash -c 'test $$(git status --porcelain | wc -l) == 0'
test:
	go get -t ./...
	go get github.com/golang/lint/golint
	go test -race ./...
	go vet ./...
	golint ./...

dev-test:
	go test -race ./...
