TaskCluster Worker
==================

<img src="https://tools.taskcluster.net/lib/assets/taskcluster-120.png" />
[![Build Status](https://travis-ci.org/taskcluster/taskcluster-worker.svg?branch=master)](http://travis-ci.org/taskcluster/taskcluster-worker)
[![GoDoc](https://godoc.org/github.com/taskcluster/taskcluster-worker?status.svg)](https://godoc.org/github.com/taskcluster/taskcluster-worker)
[![Coverage Status](https://coveralls.io/repos/taskcluster/taskcluster-worker/badge.svg?branch=master&service=github)](https://coveralls.io/github/taskcluster/taskcluster-worker?branch=master)
[![License](https://img.shields.io/badge/license-MPL%202.0-orange.svg)](http://mozilla.org/MPL/2.0)

A worker for TaskCluster, written in go.

This is our next generation worker, that has a pluggable architecture for
adding support for new engines (think Docker™ engine, Windows™ native engine,
OS X™ native engine, KVM™/Xen™ engine) and adding engine-independent plugins
(think livelogs, caches/volumes, auth proxies, interactive ssh/vnc).

Architecture
------------

See https://docs.taskcluster.net/manual/execution/workers/taskcluster-worker

Installing From Binary
----------------------

See https://github.com/taskcluster/taskcluster-worker/releases

Installing From Source
----------------------

1) [Install go 1.6](https://golang.org/doc/install) or higher
2) `go get -u -t -d github.com/taskcluster-worker/...`
3) `cd "${GOPATH}/src/github.com/taskcluster-worker"`
4) `make rebuild`

Testing
-------

```
make rebuild
```

Releasing
---------

Simply create a tag, and push to github.

```
git tag v1.0.3
git push --tags
```

Freezing Dependencies
---------------------

We are currently using [godep](https://github.com/tools/godep) to vendor dependencies.

```
# install godep tool
go get -u github.com/tools/godep
# copy vendored dependencies into your GOPATH
godep restore ./...
# change versions
cd ../jsonschema2go
git reset --hard fa5483ebd1cf3c73374e815f0befaba6184f3090
cd ../taskcluster-worker
# save changes
godep save github.com/taskcluster/jsonschema2go/...
# add changes
git add Godeps/ vendor/
# check changes look correct
git diff --cached
# commit changes
git commit -m "Froze jsonschema2go at revision fa5483ebd1cf3c73374e815f0befaba6184f3090"
```

Updating Dependencies
---------------------

The simplest is probably:

```
# install godep tool
go get -u github.com/tools/godep
# copy vendored dependencies into your GOPATH
godep restore ./...
# update versions
go get -u -t ./...
# save changes
godep save ./...
# add changes
git add Godeps/ vendor/
# check changes look correct
git diff --cached
# commit changes
git commit -m "Updated all go package dependencies to latest versions"
```

Contributing
------------

We welcome Pull Requests and Issues!

Find us in `#taskcluster-worker` on `irc.mozilla.org`
