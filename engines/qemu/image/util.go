package image

import (
	"io"
	"os"
)

// copyFile copies source to destination.
func copyFile(source, target string) error {
	// Open input file
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	// Create target file
	output, err := os.Create(target)
	if err != nil {
		return err
	}

	// Copy data
	_, err = io.Copy(output, input)
	if err != nil {
		output.Close()
		return err
	}

	// Close output file
	err = output.Close()
	if err != nil {
		return err
	}

	return nil
}
