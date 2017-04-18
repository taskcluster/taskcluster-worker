// Package fakequeue provides a fake implementation of taskcluster-queue in
// golang, The FakeQueue server stores tasks in-memory, it doesn't validate
// authentication, but implements most end-points correctly.
//
// The aim of this package is to facilitate integration tests to be executed
// without dependency on a production deployment of taskcluster-queue. Running
// integration tests against production is important, but also slow, so being
// able run them locally without credentials is very nice.
package fakequeue

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("fakequeue")
