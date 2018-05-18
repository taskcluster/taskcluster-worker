package dockerengine

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DataDog/zstd"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/fetcher"
)

type imageFetchContext struct {
	*runtime.TaskContext
}

func (ctx imageFetchContext) Progress(description string, percent float64) {
	ctx.Log(fmt.Sprintf("Progress: %s %f", description, percent))
}

type dockerClient struct {
	*docker.Client

	m     sync.Mutex
	cache map[string]*docker.Image
}

func (d *dockerClient) PullImageFromRepository(context *runtime.TaskContext, name string) (*docker.Image, error) {
	d.m.Lock()
	defer d.m.Unlock()

	if image, ok := d.cache[name]; ok {
		return image, nil
	}

	const dockerPullImageInactivityTimeout = 60 * time.Second

	context.Log(fmt.Sprintf("Downloading image %s", name))

	repo, tag := docker.ParseRepositoryTag(name)
	index := strings.Index(name, "@")
	if index != -1 {
		repo += name[index:]
	}

	err := d.PullImage(docker.PullImageOptions{
		Repository:        repo,
		Tag:               tag,
		OutputStream:      context.LogDrain(),
		InactivityTimeout: dockerPullImageInactivityTimeout,
		Context:           context,
	}, docker.AuthConfiguration{})

	if err != nil {
		return nil, errors.Wrap(err, "PullImage failed")
	}

	image, err := d.InspectImage(name)
	if err == nil {
		d.cache[name] = image
	}

	return image, err
}

// PullImageFromArtifact downloads a saved docker image as a Taskcluster artifact
// and loads it into the docker images. To acomplish it, we redirect the downloaded artifact
// output stream to docker.LoadImage and then, to extract the image name, we redirect the the output
// of the LoadImage method to a json decoder.
func (d *dockerClient) PullImageFromArtifact(context *runtime.TaskContext, options interface{}) (*docker.Image, error) {
	ctx := imageFetchContext{
		TaskContext: context,
	}

	ref, err := fetcher.Artifact.NewReference(ctx, options)
	if err != nil {
		return nil, err
	}

	d.m.Lock()
	defer d.m.Unlock()

	if image, ok := d.cache[ref.HashKey()]; ok {
		return image, nil
	}

	tempDir := filepath.Join(os.TempDir(), context.TaskID)
	if err = os.MkdirAll(tempDir, 0700); err != nil {
		return nil, errors.Wrap(err, "Error creating temporary directory")
	}
	defer os.RemoveAll(tempDir)

	tempfile, err := ioutil.TempFile(tempDir, "tc")
	if err != nil {
		return nil, errors.Wrap(err, "Error creating temporary file")
	}
	defer tempfile.Close()
	defer os.Remove(tempfile.Name())

	err = ref.Fetch(ctx, &fetcher.FileReseter{
		File: tempfile,
	})
	if err != nil {
		return nil, err
	}

	_, err = tempfile.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	zstdStream := zstd.NewReader(tempfile)
	tarFile, err := os.Create(filepath.Join(tempDir, "image.tar"))
	if err != nil {
		return nil, errors.Wrap(err, "Error creating tar file")
	}
	defer tarFile.Close()
	_, err = io.Copy(tarFile, zstdStream)
	if err != nil {
		return nil, errors.Wrap(err, "Error decompressing zst file")
	}

	imageName := strings.ToLower(ref.HashKey())
	editedTar, err := d.renameImageInTarball(imageName, tarFile.Name())

	if err != nil {
		return nil, err
	}

	defer os.Remove(editedTar)
	editedTarFile, err := os.Open(editedTar)
	if err != nil {
		return nil, errors.Wrap(err, "Error opening tar file")
	}
	defer editedTarFile.Close()

	err = d.LoadImage(docker.LoadImageOptions{
		InputStream:  editedTarFile,
		OutputStream: context.LogDrain(),
		Context:      context,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Error loading docker image")
	}

	image, err := d.InspectImage(imageName)
	if err == nil {
		d.cache[ref.HashKey()] = image
	}

	return image, err
}

// This mimics docker-worker counterpart function
func (d *dockerClient) renameImageInTarball(imageName, tarball string) (target string, err error) {
	if err = runtime.Untar(tarball); err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Failed to untar %s", tarball))
		return
	}

	dir := strings.Replace(tarball, filepath.Ext(tarball), "", 1)
	target = filepath.Join(filepath.Dir(dir), "image-edited.tar")
	debug("Edited tar path: " + target)
	reposFile, err := os.Open(filepath.Join(dir, "repositories"))
	if err != nil {
		err = errors.Wrap(err, "Repositories file not found")
		return
	}
	defer reposFile.Close()

	repoInfo := make(map[string]map[string]string)
	if err = json.NewDecoder(reposFile).Decode(&repoInfo); err != nil {
		err = errors.Wrap(err, "Invalid repositories file")
		return
	}

	newRepoInfo := make(map[string]map[string]string)
	var oldTag string
	for _, v := range repoInfo {
		newRepoInfo[imageName] = v
		for k := range v {
			oldTag = k
			break
		}
		break
	}

	if oldTag != "latest" {
		newRepoInfo[imageName]["latest"] = newRepoInfo[imageName][oldTag]
		delete(newRepoInfo[imageName], oldTag)
	}

	data, err := json.Marshal(&newRepoInfo)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Error encoding json:\n%v", newRepoInfo))
		return
	}

	err = ioutil.WriteFile(reposFile.Name(), data, 0600)
	if err != nil {
		err = errors.Wrap(err, fmt.Sprintf("Error writing %s", reposFile.Name()))
		return
	}

	manifestpPath := filepath.Join(dir, "manifest.json")

	// try to find manifest.json, if it is not present, it is probably
	// a legacy image format
	if _, err = os.Stat(manifestpPath); err != nil {
		debug("Can't open " + manifestpPath + ". Assuming a legacy format")
		err = runtime.Tar(dir, target)
		return
	}

	manifestFile, err := os.Open(manifestpPath)
	if err != nil {
		err = errors.Wrap(err, "Can't open "+manifestpPath)
		return
	}
	defer manifestFile.Close()

	var manifest []map[string]interface{}
	if err = json.NewDecoder(manifestFile).Decode(&manifest); err != nil {
		err = errors.Wrap(err, "Failed to read decode the manifest file")
		return
	}

	if len(manifest) > 1 {
		err = errors.New("Invalid manifest file")
		return
	}

	manifest[0]["RepoTags"] = []string{fmt.Sprintf("%s:latest", imageName)}
	data, err = json.Marshal(&manifest)
	if err != nil {
		err = errors.Wrap(err, "Error marshaling data")
		return
	}

	if err = ioutil.WriteFile(manifestFile.Name(), data, 0600); err != nil {
		err = errors.Wrap(err, "Error writing manifest file")
		return
	}

	err = runtime.Tar(dir, target)
	return
}
