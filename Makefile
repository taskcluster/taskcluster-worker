# A minimalistic makefile for common tasks.
# We're not supposed to use makefiles with go, but sometimes it's hard to
# remember the right command. So let's keep this simple, just targets and
# commands.

# Ensure go-extpoints and go-import-subtree are available for go generate
export PATH := $(GOPATH)/bin:$(PATH)

all: build
build:
	go fmt ./...
	go install ./...

generate:
	go get github.com/progrium/go-extpoints
	go get github.com/jonasfj/go-import-subtree
	go generate ./...
	go fmt ./...
	# report if this resulted in any code changes
	git status --porcelain {engines,plugins}/extpoints/extpoints.go

rebuild: generate build test

check: test
test:
	go test -race ./...
	go vet ./...
	# tests should fail if go generate or go fmt results in uncommitted code
	test `git status --porcelain | wc -l` == 0
