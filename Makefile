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
CGO_ENABLED := 1
LDFLAGS := "-X github.com/taskcluster/taskcluster-worker/commands/version.version=`git tag -l 'v*.*.*' --points-at HEAD | head -n1` \
						-X github.com/taskcluster/taskcluster-worker/commands/version.revision=`git rev-parse HEAD`"

.PHONY: all prechecks build rebuild check test dev-test tc-worker-env tc-worker tc-worker-env-tests

all: rebuild

prechecks:
	@test -n "$(GO_VERSION)" || (echo "Could not find go compiler, 'go version' returned no output" && false)
	@test "$(GO_MAJ)" == "go1" || (echo "Require go version 1.x, where x >= 8; however found '$(GO_VERSION)'" && false)
	@test "0$(GO_MIN)" -ge 8 || (echo "Require go version 1.x, where x>=8; however found '$(GO_VERSION)'" && false)

build:
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags $(LDFLAGS)

install:
	CGO_ENABLED=$(CGO_ENABLED) go install -ldflags $(LDFLAGS)

rebuild: prechecks build test lint

reinstall: prechecks install test lint

check: test
	# tests should fail if go fmt results in uncommitted code
	@if [ $$(git status --porcelain | wc -l) -ne 0 ]; then \
		echo "go fmt results in changes"; \
		git diff; \
		exit 1; \
	fi
test:
	# should run with -tags=system at some point..... i.e.:
	# go test -tags=system -v -race $$(go list ./... | grep -v /vendor/)
	CGO_ENABLED=$(CGO_ENABLED) go test -v -race $$(go list ./... | grep -v /vendor/)

dev-test:
	CGO_ENABLED=$(CGO_ENABLED) go test -race $$(go list ./... | grep -v /vendor/)

tc-worker-env:
	docker build -t taskcluster/tc-worker-env -f tc-worker-env.Dockerfile .

tc-worker:
	CGO_ENABLED=$(CGO_ENABLED) GOARCH=amd64 ./docker-tests.sh go build
	docker build -t taskcluster/tc-worker -f tc-worker.Dockerfile .

tc-worker-env-tests:
	@if [ -z "$TASKID" ]; then \
		echo "This command should only be used in the CI environment"; \
		exit 1 ; \
	fi
	@echo '### Pulling tc-worker-env'
	@docker pull taskcluster/tc-worker-env > /dev/null
	@echo '### Running govendor sync'
	@docker run \
		--tty --rm --privileged \
		-e DEBUG -e GOARCH=$(GOARCH) -e CGO_ENABLED=$(CGO_ENABLED) \
		-v $$(pwd):/go/src/github.com/taskcluster/taskcluster-worker/ \
		taskcluster/tc-worker-env \
		govendor sync
	@echo '### Downloading test images'
	@./engines/qemu/test-image/download.sh
	@echo '### Running tests'
	@docker run \
		--tty --rm --privileged \
		-e DEBUG -e GOARCH=$(GOARCH) -e CGO_ENABLED=$(CGO_ENABLED) \
		-v $$(pwd):/go/src/github.com/taskcluster/taskcluster-worker/ \
		taskcluster/tc-worker-env \
		go test -timeout 20m -race -tags 'qemu network system native' -p 1 -v \
		$$(find -name '*_test.go' | xargs dirname | sort | uniq)

lint:
	go get github.com/alecthomas/gometalinter
	gometalinter --install
	# not enabled: aligncheck, deadcode, dupl, errcheck, gas, gocyclo, structcheck, unused, varcheck
	# Disabled: testify, test (these two show test errors, hence, they run tests)
	# Disabled: gotype (same as go compiler, also it has issues and was recently removed)
	gometalinter -j4 --deadline=30m --line-length=180 --vendor --vendored-linters --disable-all \
		--enable=goconst \
		--enable=gofmt \
		--enable=goimports \
		--enable=golint \
		--enable=gosimple \
		--enable=ineffassign \
		--enable=interfacer \
		--enable=lll \
		--enable=misspell \
		--enable=staticcheck \
		--enable=unconvert \
		--enable=vet \
		--enable=vetshadow \
		--tests ./...
