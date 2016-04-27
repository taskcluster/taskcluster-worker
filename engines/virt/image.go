package virt

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// ExtractImage will extract the "disk" file from a tar archive using GNU tar
// ensure that sparse entries will be extracted as sparse files.
//
// Returns a MalformedPayloadError if we believe extraction failed due to a
// badly formatted image.
func ExtractImage(inputFile, outputFile string) error {
	output, err := os.OpenFile(outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("Failed to create file for image extraction: %s", err)
	}

	// Using tar so we get sparse files
	tar := exec.Command("tar", "-xOf", inputFile, "disk.img")
	tar.Stdin = nil
	tar.Stderr = nil
	tar.Stdout = output
	// TODO: Enforce some maximum time on the extraction
	// TODO: Restrict file to some maximum size

	err = tar.Run()
	if err != nil {
		return engines.NewMalformedPayloadError(
			"Failed to extract 'disk' file from image archieve, error: ", err,
		)
	}
	return nil
}
