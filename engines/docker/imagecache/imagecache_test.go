// +build linux,docker

package imagecache

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DataDog/zstd"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/require"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/gc"
	"github.com/taskcluster/taskcluster-worker/runtime/mocks"
)

const dockerSocket = "/var/run/docker.sock"
const testImage = "alpine:3.4"

func TestImageCache(t *testing.T) {
	// Skip if we don't have a docker socket
	info, err := os.Stat(dockerSocket)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Skip("didn't find docker socket at:", dockerSocket)
	}

	// Create docker client
	client, err := docker.NewClient("unix://" + dockerSocket)
	require.NoError(t, err, "failed to create docker client")

	// Create GarbageCollector and dispose all resources when done
	tracker := gc.New("/", 0, 0)
	defer tracker.CollectAll()

	// Remove the testImage in case it's already present
	client.RemoveImage(testImage)

	// Create ImageCache
	ic := New(client, tracker, mocks.NewMockMonitor(true))

	debug("### Pull Image")
	// Define function to test pulling the image to an empty cache
	testPullImage := func() {
		debug(" - Create TaskContext")
		logFile := filepath.Join(os.TempDir(), slugid.Nice())
		defer os.Remove(logFile)
		ctx, controller, err := runtime.NewTaskContext(logFile, runtime.TaskInfo{})
		require.NoError(t, err)
		defer controller.Dispose()

		debug(" - Download the test image")
		ih, err := ic.Require(ctx, testImage)
		require.NoError(t, err)
		require.NotEmpty(t, ih.ImageName)
		require.Equal(t, testImage, ih.ImageName)

		debug(" - Check that disk size works")
		size, err := ih.Resource().DiskSize()
		require.NoError(t, err)
		require.NotZero(t, size)

		debug(" - Test that the image is present")
		_, err = client.InspectImage(ih.ImageName)
		require.NoError(t, err)

		debug(" - Release the image handle")
		ih.Release()

		debug(" - Close the log")
		controller.CloseLog()

		debug(" - Read the log")
		rc, err := ctx.NewLogReader()
		require.NoError(t, err)
		defer rc.Close()
		data, err := ioutil.ReadAll(rc)
		require.NoError(t, err)
		debug("log:\n%s", string(data))
		require.Contains(t, string(data), "Pulling image")
	}
	testPullImage()

	debug("### Pull Cached Image")
	{
		debug(" - Create TaskContext")
		logFile := filepath.Join(os.TempDir(), slugid.Nice())
		defer os.Remove(logFile)
		ctx, controller, err := runtime.NewTaskContext(logFile, runtime.TaskInfo{})
		require.NoError(t, err)
		defer controller.Dispose()

		debug(" - Download the test image")
		ih, err := ic.Require(ctx, testImage)
		require.NoError(t, err)
		require.NotEmpty(t, ih.ImageName)
		require.Equal(t, testImage, ih.ImageName)

		debug(" - Release the image handle")
		ih.Release()

		debug(" - Close the log")
		controller.CloseLog()

		debug(" - Read the log")
		rc, err := ctx.NewLogReader()
		require.NoError(t, err)
		defer rc.Close()
		data, err := ioutil.ReadAll(rc)
		require.NoError(t, err)
		debug("log:\n%s", string(data))
		require.NotContains(t, string(data), "Pulling image")
	}

	debug("### Collect All Garbage")
	{
		debug("running garbage collector")
		err = tracker.CollectAll()
		require.NoError(t, err)
		_, err = client.InspectImage(testImage)
		require.Error(t, err)
	}

	debug("### Pull Image Again")
	testPullImage() // call the function we used before

	debug("### Setup image host on localhost")
	var imageBlob []byte
	var imageHash string
	{
		b := bytes.NewBuffer(nil)
		err := client.ExportImage(docker.ExportImageOptions{
			InactivityTimeout: 60 * time.Second,
			Name:              testImage,
			OutputStream:      b,
		})
		require.NoError(t, err)
		imageBlob, err = zstd.Compress(nil, b.Bytes())
		require.NoError(t, err)
		h := sha256.New()
		h.Write(imageBlob)
		imageHash = hex.EncodeToString(h.Sum(nil))
	}
	// Start server on localhost returning the imageBlob
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		debug("-> %s %s", r.Method, r.URL.Path)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/image-blob": // return the test image
			debug("<- image blob")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(imageBlob)))
			w.WriteHeader(http.StatusOK)
			w.Write(imageBlob)
		case "/invalid-image-blob": // return an invalid image, by renaming a file :)
			debug("<- invalid image blob")
			w.WriteHeader(http.StatusOK)
			ir := zstd.NewReader(bytes.NewReader(imageBlob))
			defer ir.Close()
			iw := zstd.NewWriter(w)
			defer iw.Close()
			rwerr := rewriteTarStream(ir, iw, func(hdr *tar.Header, r io.Reader) (*tar.Header, io.Reader, error) {
				switch hdr.Name {
				case "repositories":
					hdr.Name = "manifest.json" // probably this will cause some issues :)
					return hdr, r, nil
				case "manifest.json":
					return nil, nil, nil
				default:
					return hdr, r, nil
				}
			})
			require.NoError(t, rwerr)
		case "/untagged-image-blob": // return an image without any tags
			debug("<- untagged image blob")
			w.WriteHeader(http.StatusOK)
			ir := zstd.NewReader(bytes.NewReader(imageBlob))
			defer ir.Close()
			iw := zstd.NewWriter(w)
			defer iw.Close()
			rwerr := rewriteTarStream(ir, iw, func(hdr *tar.Header, r io.Reader) (*tar.Header, io.Reader, error) {
				switch hdr.Name {
				case "repositories":
					return nil, nil, nil
				case "manifest.json":
					return nil, nil, nil
				default:
					return hdr, r, nil
				}
			})
			require.NoError(t, rwerr)
		default: // return 404
			debug("<- 404")
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer s.Close()

	debug("### Fetch Image")
	fetchImage := func() {
		debug(" - Create TaskContext")
		logFile := filepath.Join(os.TempDir(), slugid.Nice())
		defer os.Remove(logFile)
		ctx, controller, err := runtime.NewTaskContext(logFile, runtime.TaskInfo{})
		require.NoError(t, err)
		defer controller.Dispose()
		defer controller.CloseLog()

		debug(" - Parse image payload")
		var imagePayload interface{}
		err = json.Unmarshal([]byte(`{
			"url": "`+s.URL+"/image-blob"+`",
			"sha256": "`+imageHash+`"
		}`), &imagePayload)
		require.NoError(t, err)

		debug(" - Download the test image")
		ih, err := ic.Require(ctx, imagePayload)
		require.NoError(t, err)
		require.NotEmpty(t, ih.ImageName)
		require.NotEqual(t, testImage, ih.ImageName)

		debug(" - Check that disk size works")
		size, err := ih.Resource().DiskSize()
		require.NoError(t, err)
		require.NotZero(t, size)

		debug(" - Test that the image is present")
		_, err = client.InspectImage(ih.ImageName)
		require.NoError(t, err)

		debug(" - Release the image handle")
		ih.Release()

		debug(" - Close the log")
		controller.CloseLog()

		debug(" - Read the log")
		rc, err := ctx.NewLogReader()
		require.NoError(t, err)
		defer rc.Close()
		data, err := ioutil.ReadAll(rc)
		require.NoError(t, err)
		debug("log:\n%s", string(data))
		require.Contains(t, string(data), "Fetching image")
	}
	fetchImage()

	debug("### Fetch Cached Image")
	{
		debug(" - Create TaskContext")
		logFile := filepath.Join(os.TempDir(), slugid.Nice())
		defer os.Remove(logFile)
		ctx, controller, err := runtime.NewTaskContext(logFile, runtime.TaskInfo{})
		require.NoError(t, err)
		defer controller.Dispose()

		debug(" - Parse image payload")
		var imagePayload interface{}
		err = json.Unmarshal([]byte(`{
			"url": "`+s.URL+"/image-blob"+`",
			"sha256": "`+imageHash+`"
		}`), &imagePayload)
		require.NoError(t, err)

		debug(" - Download the test image")
		ih, err := ic.Require(ctx, imagePayload)
		require.NoError(t, err)
		require.NotEmpty(t, ih.ImageName)
		require.NotEqual(t, testImage, ih.ImageName)

		debug(" - Test that the image is present")
		_, err = client.InspectImage(ih.ImageName)
		require.NoError(t, err)

		debug(" - Release the image handle")
		ih.Release()

		debug(" - Close the log")
		controller.CloseLog()

		debug(" - Read the log")
		rc, err := ctx.NewLogReader()
		require.NoError(t, err)
		defer rc.Close()
		data, err := ioutil.ReadAll(rc)
		require.NoError(t, err)
		debug("log:\n%s", string(data))
		require.NotContains(t, string(data), "Fetching image")
	}

	debug("### Collect All Garbage")
	{
		debug("running garbage collector")
		err = tracker.CollectAll()
		require.NoError(t, err)
		_, err = client.InspectImage(testImage)
		require.Error(t, err)
	}

	debug("### Fetch Image Again")
	fetchImage()

	debug("### Fetch Missing Image")
	{
		debug(" - Create TaskContext")
		logFile := filepath.Join(os.TempDir(), slugid.Nice())
		defer os.Remove(logFile)
		ctx, controller, err := runtime.NewTaskContext(logFile, runtime.TaskInfo{})
		require.NoError(t, err)
		defer controller.Dispose()
		defer controller.CloseLog()

		debug(" - Parse image payload")
		var imagePayload interface{}
		err = json.Unmarshal([]byte(`{
			"url": "`+s.URL+"/missing-image-blob"+`"
		}`), &imagePayload)
		require.NoError(t, err)

		debug(" - Download the test image")
		_, err = ic.Require(ctx, imagePayload)
		require.Error(t, err)
		_, ok := runtime.IsMalformedPayloadError(err)
		require.True(t, ok, "expected a MalformedPayloadError")
	}

	debug("### Fetch Invalid Image")
	{
		debug(" - Create TaskContext")
		logFile := filepath.Join(os.TempDir(), slugid.Nice())
		defer os.Remove(logFile)
		ctx, controller, err := runtime.NewTaskContext(logFile, runtime.TaskInfo{})
		require.NoError(t, err)
		defer controller.Dispose()
		defer controller.CloseLog()

		debug(" - Parse image payload")
		var imagePayload interface{}
		err = json.Unmarshal([]byte(`{
			"url": "`+s.URL+"/invalid-image-blob"+`"
		}`), &imagePayload)
		require.NoError(t, err)

		debug(" - Download the test image")
		_, err = ic.Require(ctx, imagePayload)
		require.Error(t, err)
		_, ok := runtime.IsMalformedPayloadError(err)
		require.True(t, ok, "expected a MalformedPayloadError, got %#v", err)
	}

	debug("### Fetch Untagged Image")
	{
		debug(" - Create TaskContext")
		logFile := filepath.Join(os.TempDir(), slugid.Nice())
		defer os.Remove(logFile)
		ctx, controller, err := runtime.NewTaskContext(logFile, runtime.TaskInfo{})
		require.NoError(t, err)
		defer controller.Dispose()
		defer controller.CloseLog()

		debug(" - Parse image payload")
		var imagePayload interface{}
		err = json.Unmarshal([]byte(`{
			"url": "`+s.URL+"/untagged-image-blob"+`"
		}`), &imagePayload)
		require.NoError(t, err)

		debug(" - Download the test image")
		_, err = ic.Require(ctx, imagePayload)
		require.Error(t, err)
		_, ok := runtime.IsMalformedPayloadError(err)
		require.True(t, ok, "expected a MalformedPayloadError")
	}
}
