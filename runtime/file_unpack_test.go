package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func exists(path string) error {
	_, err := os.Stat(path)
	return err
}

func readFile(t *testing.T, path string) string {
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	stat, err := os.Stat(path)
	require.NoError(t, err)

	data := make([]byte, stat.Size())
	n, err := file.Read(data)
	require.NoError(t, err)

	return string(data[:n])
}

func checkData(t *testing.T, base string) {
	if base != "" {
		base = filepath.Join("testdata", base)
	} else {
		base = "testdata"
	}
	require.NoError(t, exists(filepath.Join(base, "folder")))
	require.NoError(t, exists(filepath.Join(base, "folder/test.txt")))
	require.NoError(t, exists(filepath.Join(base, "folder/subfolder")))
	require.NoError(t, exists(filepath.Join(base, "folder/subfolder/test.txt")))
	require.Equal(t, readFile(t, filepath.Join(base, "folder/test.txt")), "This is a test.\n")
	require.Equal(t, readFile(t, filepath.Join(base, "folder/subfolder/test.txt")), "This is another test.\n")
}

func TestUnzip(t *testing.T) {
	require.NoError(t, Unzip("testdata/test.zip"))
	defer os.RemoveAll("testdata/folder")
	checkData(t, "")
}

func TestGunzipUntar(t *testing.T) {
	target, err := Gunzip("testdata/test.tar.gz")
	require.NoError(t, err)
	defer os.Remove(target)
	require.Equal(t, target, "testdata/test.tar")
	require.NoError(t, exists(target))

	require.NoError(t, Untar(target))
	defer os.RemoveAll("testdata/test")
	checkData(t, "test")

	require.NoError(t, Tar("testdata/test", "testdata/test2.tar"))
	defer os.Remove("testdata/test2.tar")

	require.NoError(t, os.RemoveAll("testdata/test"))
	require.NoError(t, Untar("testdata/test2.tar"))
	defer os.RemoveAll("testdata/test2")
	checkData(t, "test2")
}
