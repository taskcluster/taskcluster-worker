# A minimalistic makefile for common tasks.
# We're not supposed to use makefiles with go, but sometimes it's hard to
# remember the right command. So let's keep this simple, just targets and
# commands.

# Ensure bash shell! Needed for checking go version...
SHELL := /bin/bash

# For checking go compiler has a suitable version number
GO_VERSION := $(shell go version 2>/dev/null | cut -f3 -d' ')
GO_MAJ := $(shell echo "$(GO_VERSION)" | cut -f1 -d'.')
GO_MIN := $(shell echo "$(GO_VERSION)" | cut -f2 -d'.')

uname := $(shell uname)
is_darwin := $(filter Darwin,$(uname))
CGO_ENABLE := $(if $(is_darwin),1,0)

all: rebuild

prechecks:
	@test -n "$(GO_VERSION)" || (echo "Could not find go compiler, 'go version' returned no output" && false)
	@test "$(GO_MAJ)" == "go1" || (echo "Require go version 1.x, where x >= 7; however found '$(GO_VERSION)'" && false)
	@test "0$(GO_MIN)" -ge 7 || (echo "Require go version 1.x, where x>=7; however found '$(GO_VERSION)'" && false)

build:
	go fmt $$(go list ./... | grep -v /vendor/)
	CGO_ENABLED=$(CGO_ENABLE) go build

rebuild: prechecks build test

check: test
	# tests should fail if go fmt results in uncommitted code
	git status --porcelain
	/bin/bash -c 'test $$(git status --porcelain | wc -l) == 0'
	go get github.com/gordonklaus/ineffassign
	"${GOPATH}/bin/ineffassign" .

test:
	go get github.com/golang/lint/golint
	go test -v -race $$(go list ./... | grep -v /vendor/)
	go vet $$(go list ./... | grep -v /vendor/)
	go list ./... | grep -v /vendor/ | xargs -n1 golint

dev-test:
	go test -race $$(go list ./... | grep -v /vendor/)

tc-worker-env:
	docker build -t taskcluster/tc-worker-env -f tc-worker-env.Dockerfile .

tc-worker:
	CGO_ENABLED=$(CGO_ENABLE) GOARCH=amd64 go build
	docker build -t taskcluster/tc-worker -f tc-worker.Dockerfile .
