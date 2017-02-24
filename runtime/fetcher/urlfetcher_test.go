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
		err := URL.Fetch(&mockContext{context.Background(), nil, t}, s.URL+"/ok", w)
		require.NoError(t, err)
		require.Equal(t, "status-ok", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("client-error", func(t *testing.T) {
		count = 0
		w := &mockWriteSeekReseter{}
		err := URL.Fetch(&mockContext{context.Background(), nil, t}, s.URL+"/client-error", w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "client-error")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("unauthorized", func(t *testing.T) {
		count = 0
		w := &mockWriteSeekReseter{}
		err := URL.Fetch(&mockContext{context.Background(), nil, t}, s.URL+"/unauthorized", w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unauthorized")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("forbidden", func(t *testing.T) {
		count = 0
		w := &mockWriteSeekReseter{}
		err := URL.Fetch(&mockContext{context.Background(), nil, t}, s.URL+"/forbidden", w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "forbidden")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("not-found", func(t *testing.T) {
		count = 0
		w := &mockWriteSeekReseter{}
		err := URL.Fetch(&mockContext{context.Background(), nil, t}, s.URL+"/not-found", w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "not-found")
		require.Equal(t, "", w.String())
		require.Equal(t, 1, count)
	})

	t.Run("server-error", func(t *testing.T) {
		count = 0
		w := &mockWriteSeekReseter{}
		err := URL.Fetch(&mockContext{context.Background(), nil, t}, s.URL+"/server-error", w)
		require.Error(t, err)
		require.Contains(t, err.Error(), "server-error")
		require.Equal(t, "", w.String())
		require.Equal(t, maxRetries+1, count)
	})
}
