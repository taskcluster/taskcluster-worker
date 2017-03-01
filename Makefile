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

.PHONY: all prechecks build rebuild check test dev-test tc-worker-env tc-worker

all: rebuild

prechecks:
	@test -n "$(GO_VERSION)" || (echo "Could not find go compiler, 'go version' returned no output" && false)
	@test "$(GO_MAJ)" == "go1" || (echo "Require go version 1.x, where x >= 7; however found '$(GO_VERSION)'" && false)
	@test "0$(GO_MIN)" -ge 7 || (echo "Require go version 1.x, where x>=7; however found '$(GO_VERSION)'" && false)

build:
	go fmt $$(go list ./... | grep -v /vendor/)
	CGO_ENABLED=$(CGO_ENABLE) go build

rebuild: prechecks build test lint

check: test
	# tests should fail if go fmt results in uncommitted code
	git status --porcelain
	/bin/bash -c 'test $$(git status --porcelain | wc -l) == 0'
test:
	# should run with -tags=system at some point..... i.e.:
	# go test -tags=system -v -race $$(go list ./... | grep -v /vendor/)
	go test -v -race $$(go list ./... | grep -v /vendor/)

dev-test:
	go test -race $$(go list ./... | grep -v /vendor/)

tc-worker-env:
	docker build -t taskcluster/tc-worker-env -f tc-worker-env.Dockerfile .

tc-worker:
	CGO_ENABLED=$(CGO_ENABLE) GOARCH=amd64 go build
	docker build -t taskcluster/tc-worker -f tc-worker.Dockerfile .

lint:
	go get github.com/alecthomas/gometalinter
	gometalinter --install
	# not enabled: aligncheck, deadcode, dupl, errcheck, gas, gocyclo, structcheck, unused, varcheck
	$(GOPATH)/bin/gometalinter --deadline=10m --line-length=180 --vendor --vendored-linters --disable-all --enable=goconst --enable=gofmt --enable=goimports --enable=golint --enable=gosimple --enable=gotype --enable=ineffassign --enable=interfacer --enable=lll --enable=misspell --enable=staticcheck --enable=test --enable=testify --enable=unconvert --enable=vet --enable=vetshadow ./...
