# A minimalistic makefile for common tasks.
# We're not supposed to use makefiles with go, but sometimes it's hard to
# remember the right command. So let's keep this simple, just targets and
# commands.

all: build
build:
	go install ./...

generate:
	go get github.com/progrium/go-extpoints
	go get github.com/jonasfj/go-import-subtree
	go generate ./...

rebuild: generate build test

check: test
test:
	go test -race ./...
