package util

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

// Download downloads the request file in the url
func Download(url, destdir string) (string, error) {
	resp, err := http.Get(url)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	contentDisposition := resp.Header.Get("Content-Disposition")
	_, params, err := mime.ParseMediaType(contentDisposition)

	var filename string
	if err == nil {
		filename = params["filename"]
	} else {
		filename = filepath.Base(url)
	}

	filename = filepath.Join(destdir, filename)
	file, err := os.Create(filename)

	if err != nil {
		return "", err
	}

	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(filename)
		return "", err
	}

	return filename, nil
}
