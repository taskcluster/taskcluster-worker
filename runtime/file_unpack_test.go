package runtime

import (
	"os"
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

func checkData(t *testing.T) {
	require.NoError(t, exists("testdata/folder"))
	require.NoError(t, exists("testdata/folder/test.txt"))
	require.NoError(t, exists("testdata/folder/subfolder"))
	require.NoError(t, exists("testdata/folder/subfolder/test.txt"))
	require.Equal(t, readFile(t, "testdata/folder/test.txt"), "This is a test.\n")
	require.Equal(t, readFile(t, "testdata/folder/subfolder/test.txt"), "This is another test.\n")
}

func TestUnzip(t *testing.T) {
	require.NoError(t, Unzip("testdata/test.zip"))
	defer os.RemoveAll("testdata/folder")
	checkData(t)
}

func TestGunzipUntar(t *testing.T) {
	target, err := Gunzip("testdata/test.tar.gz")
	require.NoError(t, err)
	defer os.Remove(target)
	require.Equal(t, target, "testdata/test.tar")
	require.NoError(t, exists(target))

	require.NoError(t, Untar(target))
	defer os.RemoveAll("testdata/folder")
	checkData(t)
}
