// Package displayclient provides a golang implementation of websockify,
// transforming a websocket connection to an ioext.ReadWriteCloser object.
//
// The goal of this is to make it easy to interact with a VNC connection from
// a go application. Both for writing tests and command line utilities.
package displayclient

import "github.com/taskcluster/taskcluster-worker/runtime/util"

var debug = util.Debug("displayclient")
