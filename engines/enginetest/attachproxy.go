package enginetest

import (
	"bytes"
	"net/http"
	"strings"
	"sync"
)

// PingPath is the path that PingProxyPayload should hit on the proxy.
const PingPath = "/v1/ping"

// A ProxyTestCase holds information necessary to run tests that an engine
// can attach proxies, call them and forward calls correctly
type ProxyTestCase struct {
	*EngineProvider
	// A valid name for a proxy attachment
	ProxyName string
	// A task.payload as accepted by the engine, which will write "Pinging"
	// to the log, then ping the proxy given by ProxyName with GET to the path
	// "/v1/ping", and write the response to log.
	// The task payload must exit successfully if proxy response code is 200,
	// and unsuccessful if the response code is 404.
	PingProxyPayload string
}

// TestPingProxyPayload checks that PingProxyPayload works as defined
func (c *ProxyTestCase) TestPingProxyPayload() {
	debug("### TestPingProxyPayload")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.PingProxyPayload)

	pinged := false
	pingMethod := "-"
	pingPath := ""
	err := r.sandboxBuilder.AttachProxy(c.ProxyName, http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		pinged = true
		pingMethod = r.Method
		pingPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Yay, you managed to ping the end-point, secret=42!!!"))
	}))
	nilOrPanic(err, "Error failed to AttachProxy")

	result := r.buildRunSandbox()
	log := r.ReadLog()

	assert(result, "PingProxyPayload exited unsuccessfully, log: ", log)
	assert(pinged, "PingProxyPayload didn't call the attachedProxy, log: ", log)
	assert(pingMethod == "GET" || pingMethod == "",
		"PingProxyPayload pinged with method: ", pingMethod)
	assert(pingPath == PingPath, "PingProxyPayload pinged path: ", pingPath)
	assert(strings.Contains(log, "secret=42"),
		"Didn't find secret=42 from ping response in log", log)
}

// TestPing404IsUnsuccessful checks that 404 returns unsuccessful
func (c *ProxyTestCase) TestPing404IsUnsuccessful() {
	debug("### TestPing404IsUnsuccessful")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.PingProxyPayload)

	pinged := false
	pingPath := ""
	err := r.sandboxBuilder.AttachProxy(c.ProxyName, http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		pinged = true
		pingPath = r.URL.Path
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Yay, you managed to ping the end-point, secret=42!!!"))
	}))
	nilOrPanic(err, "Error failed to AttachProxy")

	result := r.buildRunSandbox()
	log := r.ReadLog()

	assert(!result, "PingProxyPayload exited successfully, when we returned 404")
	assert(pinged, "PingProxyPayload didn't call the attachedProxy")
	assert(pingPath == PingPath, "PingProxyPayload pinged path: ", pingPath)
	assert(strings.Contains(log, "secret=42"),
		"Didn't find secret=42 from ping response in log", log)
}

// TestLiveLogging checks that "Pinging" is readable from log before the task
// is finished.
func (c *ProxyTestCase) TestLiveLogging() {
	debug("### TestLiveLogging")
	r := c.newRun()
	defer r.Dispose()
	r.NewSandboxBuilder(c.PingProxyPayload)

	// Read livelog until we see "Pinging"
	readPinging := make(chan struct{})
	go func() {
		r.OpenLogReader()
		buf := bytes.Buffer{}
		for !strings.Contains(buf.String(), "Pinging") {
			b := []byte{0}
			n, err := r.logReader.Read(b)
			if n != 1 {
				panic("Expected one byte to be read!")
			}
			buf.WriteByte(b[0])
			nilOrPanic(err, "Failed while reading from livelog...")
		}
		close(readPinging)
	}()

	pinged := false
	pingPath := ""
	err := r.sandboxBuilder.AttachProxy(c.ProxyName, http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		// Wait until readPinging is done, before we proceed to reply
		<-readPinging
		pinged = true
		pingPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte("Yay, you managed to ping the end-point, secret=42!!!"))
	}))
	nilOrPanic(err, "Error failed to AttachProxy")

	result := r.buildRunSandbox()
	log := r.ReadLog()

	assert(result, "PingProxyPayload exited unsuccessfully")
	assert(pinged, "PingProxyPayload didn't call the attachedProxy")
	assert(pingPath == PingPath, "PingProxyPayload pinged path: ", pingPath)
	assert(strings.Contains(log, "secret=42"),
		"Didn't find 'secret=42' from ping response in log", log)
	assert(strings.Contains(log, "Pinging"), "Didn't find 'Pinging' in log", log)
}

// TestParallelPings checks that two parallel pings is possible when running
// two engines next to each other.
func (c *ProxyTestCase) TestParallelPings() {
	debug("### TestParallelPings")
	// TODO: Make two sandboxes. inside http.handler use a WaitGroup to ensure
	// that both sandboxes has sent their request to the proxy before either
	// one of the two handlers respond.
}

// Test runs all tests for the ProxyTestCase is parallel
func (c *ProxyTestCase) Test() {
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() { c.TestPingProxyPayload(); wg.Done() }()
	go func() { c.TestPing404IsUnsuccessful(); wg.Done() }()
	go func() { c.TestLiveLogging(); wg.Done() }()
	go func() { c.TestParallelPings(); wg.Done() }()
	wg.Wait()
}
