package vm

// An Image provides an instance of a virtual machine image that a virtual
// machine can be started from.
type Image interface {
	DiskFile() string // Primary disk file to be used as boot disk.
	Format() string   // Image format 'qcow2', 'raw', etc.
	Machine() Machine // Machine configuration.
	Release()         // Free resources held by this image instance.
}

// A MutableImage is an instance of a virtual machine image similar to
// to Image, except that it can also be packaged into a compressed image
// tar archieve as used for input by image manager.
type MutableImage interface {
	Image
	Package(targetFile string) error // Package the as lz4 compressed tar archieve
}
