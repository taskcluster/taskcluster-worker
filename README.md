TaskCluster Worker
==================

[![logo](https://tools.taskcluster.net/lib/assets/taskcluster-120.png)](https://tools.taskcluster.net/lib/assets/taskcluster-120.png)

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

1. [Install go 1.7](https://golang.org/doc/install) or higher
2. `go get -u -t -d github.com/taskclustertaskcluster-worker/...`
3. `cd "${GOPATH}/src/github.com/taskcluster-worker"`
4. `go get -u github.com/kardianos/govendor`
5. `govendor sync`
6. `make rebuild`

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

You need [govendor](https://github.com/kardianos/govendor) to manage vendor dependencies.

```
govendor sync
```

Adding Dependencies
---------------------

```
go get <package>
govendor add +external
git add vendor/vendor.json
git commit -m 'My new package.'
```

Updating Dependencies
---------------------

```
go get -u -t ./...   # update versions
govendor update
```

Contributing
------------

We welcome Pull Requests and Issues!

Find us in `#taskcluster-worker` on `irc.mozilla.org`

