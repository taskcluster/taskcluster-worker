package fetcher

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUrlHashFetcher(t *testing.T) {
	ctx := &mockContext{
		Context: context.Background(),
	}

	// HACK: Reduce backOff.MaxDelay for the duration of this test
	maxDelay := backOff.MaxDelay
	backOff.MaxDelay = 100 * time.Millisecond
	defer func() { backOff.MaxDelay = maxDelay }()

	var h hash.Hash
	h = md5.New()
	h.Write([]byte("status-ok"))
	md5ok := hex.EncodeToString(h.Sum(nil))
	h = sha1.New()
	h.Write([]byte("status-ok"))
	sha1ok := hex.EncodeToString(h.Sum(nil))
	h = sha256.New()
	h.Write([]byte("status-ok"))
	sha256ok := hex.EncodeToString(h.Sum(nil))
	h = sha512.New()
	h.Write([]byte("status-ok"))
	sha512ok := hex.EncodeToString(h.Sum(nil))

	// Count number of request and setup a test server
	count := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(200)
			w.Write([]byte("status-ok"))

		case "/streaming":
			// streaming without Content-Length
			w.WriteHeader(200)
			for i := 0; i < 10; i++ {
				time.Sleep(10 * time.Millisecond)
				w.Write([]byte("hello\n"))
				w.(http.Flusher).Flush()
			}

		case "/slow":
			w.Header().Set("Content-Length", "60")
			w.WriteHeader(200)
			debug("starting slow response")
			for i := 0; i < 10; i++ {
				time.Sleep(100 * time.Millisecond)
				w.Write([]byte("hello\n"))
				w.(http.Flusher).Flush()
			}
			debug("finished slow response")

		case "/client-error":
			w.WriteHeader(400)
			w.Write([]byte("client-error"))

		case "/unauthorized":
			w.WriteHeader(401)
			w.Write([]byte("unauthorized"))

		case "/forbidden":
			w.WriteHeader(403)
			w.Write([]byte("forbidden"))

		case "/not-found":
			w.WriteHeader(404)
			w.Write([]byte("not-found"))

		case "/server-error":
			w.WriteHeader(500)
			w.Write([]byte("server-error"))

		default:
			panic("Unhandled path: " + r.URL.Path)
		}
	}))
	defer s.Close()

	t.Run("status-ok", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url": s.URL + "/ok",
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "status-ok", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("status-ok with md5", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url": s.URL + "/ok",
			"md5": md5ok,
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "status-ok", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("status-ok with sha1", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url":  s.URL + "/ok",
			"sha1": sha1ok,
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "status-ok", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("status-ok with sha256", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url":    s.URL + "/ok",
			"sha256": sha256ok,
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "status-ok", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("status-ok with wrong sha256", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url":    s.URL + "/ok",
			"sha256": sha256ok[:60] + "ffff",
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "SHA256")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("status-ok with sha512", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url":    s.URL + "/ok",
			"sha512": sha512ok,
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "status-ok", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("status-ok with hashes", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url":    s.URL + "/ok",
			"md5":    md5ok,
			"sha1":   sha1ok,
			"sha256": sha256ok,
			"sha512": sha512ok,
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "status-ok", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("streaming-ok", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url": s.URL + "/streaming",
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.NoError(t, err)
		require.Contains(t, w.String(), "hello")
		require.Equal(t, 1, count)
	})

	t.Run("slow progress reports", func(t *testing.T) {
		// Spoof progress report for this test case
		origProgressReportInterval := progressReportInterval
		progressReportInterval = 100 * time.Millisecond
		defer func() {
			progressReportInterval = origProgressReportInterval
		}()

		ctx2 := &mockContext{Context: context.Background()}
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx2, map[string]interface{}{
			"url": s.URL + "/slow",
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx2, w)
		require.NoError(t, err)
		require.Equal(t, 1, count)
		require.True(t, len(ctx2.ProgressReports()) > 3, "expected more than 3 progress reports")
		var prev float64
		for _, report := range ctx2.ProgressReports() {
			require.True(t, prev <= report, "Reporting shouldn't go backwards")
			prev = report
		}
	})

	t.Run("client-error", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url": s.URL + "/client-error",
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "client-error")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("unauthorized", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url": s.URL + "/unauthorized",
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unauthorized")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("forbidden", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url": s.URL + "/forbidden",
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "forbidden")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("not-found", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url": s.URL + "/not-found",
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not-found")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("server-error", func(t *testing.T) {
		count = 0
		w := &mockWriteReseter{}
		ref, err := URLHash.NewReference(ctx, map[string]interface{}{
			"url": s.URL + "/server-error",
		})
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "server-error")
		require.Equal(t, "", w.String())
		require.Equal(t, maxRetries+1, count)
	})
}
