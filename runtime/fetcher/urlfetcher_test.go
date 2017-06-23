package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUrlFetcher(t *testing.T) {
	ctx := &mockContext{
		Context: context.Background(),
	}

	// HACK: Reduce backOff.MaxDelay for the duration of this test
	maxDelay := backOff.MaxDelay
	backOff.MaxDelay = 100 * time.Millisecond
	defer func() { backOff.MaxDelay = maxDelay }()

	// Count number of request and setup a test server
	count := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(200)
			w.Write([]byte("status-ok"))

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
		w := &mockWriteSeekReseter{}
		ref, err := URL.NewReference(ctx, s.URL+"/ok")
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.NoError(t, err)
		require.Equal(t, "status-ok", w.String())
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
		w := &mockWriteSeekReseter{}
		ref, err := URL.NewReference(ctx2, s.URL+"/slow")
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
		w := &mockWriteSeekReseter{}
		ref, err := URL.NewReference(ctx, s.URL+"/client-error")
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "client-error")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("unauthorized", func(t *testing.T) {
		count = 0
		w := &mockWriteSeekReseter{}
		ref, err := URL.NewReference(ctx, s.URL+"/unauthorized")
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unauthorized")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("forbidden", func(t *testing.T) {
		count = 0
		w := &mockWriteSeekReseter{}
		ref, err := URL.NewReference(ctx, s.URL+"/forbidden")
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "forbidden")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("not-found", func(t *testing.T) {
		count = 0
		w := &mockWriteSeekReseter{}
		ref, err := URL.NewReference(ctx, s.URL+"/not-found")
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not-found")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("server-error", func(t *testing.T) {
		count = 0
		w := &mockWriteSeekReseter{}
		ref, err := URL.NewReference(ctx, s.URL+"/server-error")
		require.NoError(t, err)
		err = ref.Fetch(ctx, w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "server-error")
		require.Equal(t, "", w.String())
		require.Equal(t, maxRetries+1, count)
	})
}
