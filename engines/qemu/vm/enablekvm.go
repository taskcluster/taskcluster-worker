// +build !vagrant

package vm

// accelerator is set to "kvm" when built for non-vagrant environment
const accelerator = "kvm"
