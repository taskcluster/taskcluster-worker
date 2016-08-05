package ioext

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestIsPlainFile(t *testing.T) {
	folder, err := ioutil.TempDir("", "ioext-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(folder)

	p := filepath.Join(folder, "test.txt")
	if err := ioutil.WriteFile(p, []byte("Hello world"), 0600); err != nil {
		t.Fatal(err)
	}

	if !IsPlainFile(p) {
		t.Error("IsPlainFile is wrong - tested a file")
	}

	f := filepath.Join(folder, "dir")
	if err := os.Mkdir(f, 0600); err != nil {
		t.Fatal(err)
	}

	if IsPlainFile(f) {
		t.Error("IsPlainFile is wrong - tested folder")
	}
}

func TestIsFileLessThan(t *testing.T) {
	folder, err := ioutil.TempDir("", "ioext-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(folder)

	p := filepath.Join(folder, "test.txt")
	if err := ioutil.WriteFile(p, []byte("Hello world"), 0600); err != nil {
		t.Fatal(err)
	}

	if IsFileLessThan(p, 11) {
		t.Error("Shouldn't be less than 11")
	}
	if !IsFileLessThan(p, 12) {
		t.Error("Should be less than 12")
	}
}
