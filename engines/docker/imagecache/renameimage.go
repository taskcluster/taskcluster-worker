package imagecache

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/pkg/errors"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
)

var allowedDockerLayerIDPattern = regexp.MustCompile(`^[a-fA-f0-9]{64}$`)

func isAllowedDockerLayerFile(name string) bool {
	if len(name) < 64 || !allowedDockerLayerIDPattern.MatchString(name[0:64]) {
		return false
	}
	switch name[64:] {
	case ".json", "/", "/json", "/layer.tar", "/VERSION":
		return true
	default:
		return false
	}
}

// renameDockerImageTarStream renames a tar-stream containing a docker image
// in the `docker save` format. The tar-stream is renamed on-the-fly without
// any files being written to disk.
func renameDockerImageTarStream(imageName string, r io.Reader, w io.Writer) error {
	// We call rewriteTarStream to rename the tar-stream on-the-fly
	nameWritten := false // check that we found 'repositories' or 'manifest.json'
	rwerr := rewriteTarStream(r, w, func(hdr *tar.Header, r io.Reader) (*tar.Header, io.Reader, error) {
		// Whenever an entry is reached we switch on the name
		switch hdr.Name {
		case "repositories", "manifest.json":
			nameWritten = true
			data, err := ioext.ReadAtMost(r, 10*1024*1024)
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to read the metadata file from docker image tar-stream")
			}
			debug("renameDockerImageTarStream(): rewriting '%s'", hdr.Name)
			switch hdr.Name {
			case "repositories":
				data, err = rewriteRepositories(imageName, data)
			case "manifest.json":
				data, err = rewriteManifest(imageName, data)
			default:
				panic(errors.New("unreachable"))
			}
			if err != nil {
				return nil, nil, err
			}
			hdr.Size = int64(len(data))
			return hdr, bytes.NewReader(data), nil
		default:
			if !isAllowedDockerLayerFile(hdr.Name) {
				debug("renameDockerImageTarStream(): illegal file '%s'", hdr.Name)
				return nil, nil, runtime.NewMalformedPayloadError(fmt.Sprintf(
					"docker image tar-ball contains illegal file: '%s', consider using an older version of docker", hdr.Name,
				))
			}
			// If file is allowed we just let it pass through
			debug("renameDockerImageTarStream(): passing through '%s'", hdr.Name)
			return hdr, r, nil
		}
	})
	// If there was no error and name wasn't written at some point, then we have problem
	if rwerr == nil && !nameWritten {
		return runtime.NewMalformedPayloadError(
			"docker image tar-ball did contain 'manifest.json' or 'repositories', docker image is invalid",
		)
	}
	return rwerr
}

// rewriteRepositories will rewrite the 'repositories' file from a docker image
// tar-stream such that the image has a single tag named <imageName>:latest
func rewriteRepositories(imageName string, data []byte) ([]byte, error) {
	// Rewrite the 'repositories' by reading it as JSON from the input stream
	var repositories map[string]map[string]string
	err := json.Unmarshal(data, &repositories)
	if err != nil {
		debug("rewriteRepositories(): JSON parse error: %s", err)
		return nil, runtime.NewMalformedPayloadError(fmt.Sprintf(
			"the 'repositories' file in the docker image tar-ball is not valid JSON, error: %s",
			err.Error(),
		))
	}
	// Find the layerID
	var layerID string
	for image, tags := range repositories {
		for tag, ID := range tags {
			debug("rewriteRepositories(): found image %s:%s -> %s", image, tag, ID)
			// if there are multiple tags pointing to different IDs that's problematic
			if layerID == "" {
				layerID = ID
			} else if layerID != ID {
				return nil, runtime.NewMalformedPayloadError(
					"the 'repositories' file in the docker image tar-ball contains multiple tags with different layer IDs",
				)
			}
		}
	}
	// If there is no imageID we have a problem
	if layerID == "" {
		debug("rewriteRepositories(): the 'repositories' file don't have any image tags")
		return nil, runtime.NewMalformedPayloadError(
			"the 'repositories' file in the docker image tar-ball did not contain any layer identifiers, image is invalid",
		)
	}
	// Create result of the rewrite and return it
	data, err = json.Marshal(map[string]map[string]string{
		imageName: {"latest": layerID}, // map from imageName to layerID
	})
	if err != nil {
		panic(errors.Wrap(err, "json.Marshal failed on map[string]map[string]string that can't happen"))
	}
	return data, nil
}

// Type to read from manifest.json
type manifestEntry struct {
	Config   string   `json:"Config,omitempty"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

// rewriteManifest will rewrite the 'manifest.json' file from a docker image
// tar-stream such that the image has a single tag named <imageName>:latest
func rewriteManifest(imageName string, data []byte) ([]byte, error) {
	// Read the manifest.json from input
	var manifest []manifestEntry
	err := json.Unmarshal(data, &manifest)
	if err != nil {
		debug("rewriteManifest(): JSON parse error: %s", err)
		return nil, runtime.NewMalformedPayloadError(fmt.Sprintf(
			"the 'manifest.json' file in the docker image tar-ball is not valid JSON, error: %s",
			err.Error(),
		))
	}
	if len(manifest) == 0 {
		debug("rewriteManifest(): manifest is missing entries")
		return nil, runtime.NewMalformedPayloadError(
			"the 'manifest.json' file in the docker image tar-ball did not contain any entries",
		)
	}
	if len(manifest) > 1 {
		debug("rewriteManifest(): manifest contains multiple entries")
		return nil, runtime.NewMalformedPayloadError(
			"the 'manifest.json' file in the docker image tar-ball contains more than one entry",
		)
	}
	// Rewrite the RepoTags and only the first entry as JSON for result
	manifest[0].RepoTags = []string{fmt.Sprintf("%s:latest", imageName)}
	data, err = json.Marshal([]manifestEntry{manifest[0]})
	if err != nil {
		panic(errors.Wrap(err, "json.Marshal failed on []manifestEntry that can't happen"))
	}
	return data, nil
}
