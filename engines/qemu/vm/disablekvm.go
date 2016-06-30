// +build vagrant

package vm

// accelerator is set to "tcg" when built for vagrant environment, this is
// because kvm isn't available inside vagrant where we run most of our tests.
const accelerator = "tcg"
