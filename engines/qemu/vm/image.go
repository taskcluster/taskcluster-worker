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
// tar archive as used for input by image manager.
type MutableImage interface {
	Image
	Package(targetFile string) error // Package the as zstd compressed tar archive
}

// imageMachinePair holds an image and a machine overwriting the built-in machine.
type imageMachinePair struct {
	Image
	machine Machine
}

func (i *imageMachinePair) Machine() Machine {
	return i.machine
}

// OverwriteMachine returns an image with a machine definition whose properties
// is overwritten by machine given here.
func OverwriteMachine(image Image, machine Machine) Image {
	return &imageMachinePair{
		Image:   image,
		machine: machine.WithDefaults(image.Machine()),
	}
}
