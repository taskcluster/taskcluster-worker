package qemuengine

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
)

type resultSet struct {
	engines.ResultSetBase
	success bool
	vm      *vm.VirtualMachine
}

func newResultSet(success bool, vm *vm.VirtualMachine) *resultSet {
	return &resultSet{
		success: success,
		vm:      vm,
	}
}

func (r *resultSet) Success() bool {
	return r.success
}

func (r *resultSet) Dispose() error {
	r.vm.Kill()
	<-r.vm.Done
	return nil
}
