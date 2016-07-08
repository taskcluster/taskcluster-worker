package qemuengine

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
)

type resultSet struct {
	engines.ResultSetBase
	success     bool
	vm          *vm.VirtualMachine
	metaService *metaservice.MetaService
}

func newResultSet(success bool, vm *vm.VirtualMachine, m *metaservice.MetaService) *resultSet {
	// Set metaService as handler (this will make proxies unreachable)
	vm.SetHTTPHandler(m)
	return &resultSet{
		success:     success,
		vm:          vm,
		metaService: m,
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
