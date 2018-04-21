package fetcher

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchAsStream(t *testing.T) {
	ctx := &fakeContext{Context: context.Background()}
	// Create a random blob
	blob := make([]byte, 16*1024+27)
	_, rerr := rand.Read(blob)
	require.NoError(t, rerr, "failed to created random data for testing")

	t.Run("hello-world", func(t *testing.T) {
		var result string
		err := FetchAsStream(ctx, &fakeReference{
			Data:  []byte("hello-world"),
			Reset: false,
			Err:   nil,
		}, func(_ context.Context, r io.Reader) error {
			b := bytes.NewBuffer(nil)
			_, cerr := io.Copy(b, r)
			result = b.String()
			return cerr
		})
		require.NoError(t, err)
		require.Equal(t, "hello-world", result)
	})

	t.Run("hello-world with Reset", func(t *testing.T) {
		var result string
		err := FetchAsStream(ctx, &fakeReference{
			Data:  []byte("hello-world"),
			Reset: true,
			Err:   nil,
		}, func(ctx context.Context, r io.Reader) error {
			b := bytes.NewBuffer(nil)
			_, cerr := io.Copy(b, r)
			if cerr != nil {
				return cerr
			}
			result = b.String()
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, "hello-world", result)
	})

	t.Run("blob", func(t *testing.T) {
		var result []byte
		err := FetchAsStream(ctx, &fakeReference{
			Data:  blob,
			Reset: false,
			Err:   nil,
		}, func(_ context.Context, r io.Reader) error {
			b := bytes.NewBuffer(nil)
			_, cerr := io.Copy(b, r)
			result = b.Bytes()
			return cerr
		})
		require.NoError(t, err)
		require.True(t, bytes.Equal(blob, result), "expected blob == result")
	})

	t.Run("blob with Reset", func(t *testing.T) {
		var result []byte
		err := FetchAsStream(ctx, &fakeReference{
			Data:  blob,
			Reset: true,
			Err:   nil,
		}, func(ctx context.Context, r io.Reader) error {
			b := bytes.NewBuffer(nil)
			_, cerr := io.Copy(b, r)
			if cerr != nil {
				return cerr
			}
			result = b.Bytes()
			return nil
		})
		require.NoError(t, err)
		require.True(t, bytes.Equal(blob, result), "expected blob == result")
	})

	t.Run("Reference with Err", func(t *testing.T) {
		berr := errors.New("my bad error")
		err := FetchAsStream(ctx, &fakeReference{
			Data:  []byte("hello-world"),
			Reset: false,
			Err:   berr,
		}, func(ctx context.Context, r io.Reader) error {
			b := bytes.NewBuffer(nil)
			_, cerr := io.Copy(b, r)
			if cerr != nil {
				return cerr
			}
			panic("this should not be reachable, as fetching failed, so should reading from io.Reader")
		})
		require.Equal(t, berr, err)
	})
}
