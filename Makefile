# A minimalistic makefile for common tasks.
# We're not supposed to use makefiles with go, but sometimes it's hard to
# remember the right command. So let's keep this simple, just targets and
# commands.

all: build
build:
	go build

generate:
	go get github.com/progrium/go-extpoints
	go generate ./...

rebuild: generate build test

check: test
test:
	go test -race ./...
