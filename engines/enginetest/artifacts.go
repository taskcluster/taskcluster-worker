package enginetest

// The ArtifactTestCase contains information sufficient to test artifact
// extration from an engine.
type ArtifactTestCase struct {
	Engine string
	// Path of a file containing the string "Hello World"
	HelloWorldFilePath string
	// Path to a file that doesn't exist, and will return ErrResourceNotFound
	FileNotFoundPath string
	// Path to a folder that doesn't exist, and will return ErrResourceNotFound
	FolderNotFoundPath string
	// Path to a folder that contains: A.txt, B.txt and C/C.txt, each containing
	// the string "Hello World"
	NestedFolderPath string
	// Payload that will generate a ResultSet containing paths described above.
	Payload string
}
