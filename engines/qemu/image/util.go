package image

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/taskcluster/go-got"
	"github.com/taskcluster/taskcluster-worker/engines"
)

// copyFile copies source to destination, and returns an error if one occurs
// during opening input or output file, copying data from input to output, or
// closing input or output file
func copyFile(source, target string) (err error) {
	var input *os.File
	var output *os.File
	input, err = os.Open(source)
	if err != nil {
		return
	}

	closeFile := func(f *os.File) {
		err2 := f.Close()
		if err == nil {
			err = err2
		}
	}

	defer closeFile(input)

	// Create target file
	output, err = os.Create(target)
	if err != nil {
		return
	}
	defer closeFile(output)

	// Copy data
	_, err = io.Copy(output, input)
	return
}

const maxRetries = 7

// DownloadImage returns a Downloader that will download the image from the
// given url. This will attempt multiple retries if necessary.
//
// If there is a non-200 response this will return a MalformedPayloadError.
func DownloadImage(url string) Downloader {
	// TODO: Add some logging, I really want to abstract away Logger
	// TODO: This method could probably exist somewhere else, in say runtime
	//       downloading from a URL to a file or stream with retries, etc. is a
	//       common thing. Using range headers for retries and getting integrity
	//       checks right is hard.
	return func(target string) error {
		// Create output file
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()

		attempt := 1
		for {
			// Move to start of file and truncate the file
			_, err = out.Seek(0, 0)
			if err != nil {
				panic("Unable to seek to file start")
			}
			err = out.Truncate(0)
			if err != nil {
				panic("Unable to truncate file")
			}

			// Create a GET request
			res, err := http.Get(url)
			if err != nil {
				goto retry
			}
			if 500 <= res.StatusCode && res.StatusCode < 600 {
				err = fmt.Errorf("Image download failed with status code: %d", res.StatusCode)
				goto retry
			}
			if res.StatusCode != 200 {
				return engines.NewMalformedPayloadError(
					"Image download failed with status code: ", res.StatusCode,
				)
			}

			// Copy response to file
			// TODO: Make integrity check with x-amx-meta-content-sha256 (if present)
			// TODO: Use range headers for retry, if supported and checksum for
			//       integrity check is present (otherwise request from start)
			_, err = io.Copy(out, res.Body)
			res.Body.Close()
			if err == nil {
				return nil
			}
		retry:
			if attempt >= maxRetries {
				return err
			}
			attempt++
			got.DefaultBackOff.Delay(attempt)
		}
	}
}
