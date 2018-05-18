package unpack

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unzip unzips a zipped file
func Unzip(filename string) error {
	// Open a zip archive for reading.
	r, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer r.Close()

	// Iterate through the files in the archive
	for _, f := range r.File {
		fileName := filepath.Join(filepath.Dir(filename), f.Name)
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(fileName, f.Mode()); err != nil {
				return err
			}
		} else {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			destFile, err := os.Create(fileName)
			if err != nil {
				return err
			}
			_, err = io.Copy(destFile, rc)
			destFile.Close()
			if err != nil {
				return err
			}
			if err = os.Chmod(fileName, f.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

// Gunzip gunzips a zipped file and returns its name
func Gunzip(filename string) (string, error) {
	reader, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	archive, err := gzip.NewReader(reader)
	if err != nil {
		return "", err
	}
	defer archive.Close()

	target := strings.TrimSuffix(filename, filepath.Ext(filename))
	writer, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer writer.Close()

	_, err = io.Copy(writer, archive)
	return target, err
}

// Untar unpacks a tar file
func Untar(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	// Open a tar archive for reading.
	t := tar.NewReader(file)

	// Iterate through the files in the archive
	for {
		hdr, err := t.Next()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return err
		}
		fileName := filepath.Join(filepath.Dir(filename), hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(fileName, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
		case tar.TypeReg:
			{
				f, err := os.Create(fileName)
				if err != nil {
					return err
				}
				_, err = io.Copy(f, t)
				f.Close()
				if err != nil {
					return err
				}
				err = os.Chmod(fileName, hdr.FileInfo().Mode())
				if err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("File type %v unsupported: %s", hdr.Typeflag, hdr.Name)
		}
	}
}
