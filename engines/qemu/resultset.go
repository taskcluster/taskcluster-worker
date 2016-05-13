package qemuengine

import "github.com/taskcluster/taskcluster-worker/engines"

type resultSet struct {
	engines.ResultSetBase
	success bool
	vm      *virtualMachine
}

func newResultSet(success bool, vm *virtualMachine) *resultSet {
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
