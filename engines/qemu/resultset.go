package qemuengine

import (
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/vm"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
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

func (r *resultSet) ExtractFile(path string) (ioext.ReadSeekCloser, error) {
	return r.metaService.GetArtifact(path)
}

func (r *resultSet) ExtractFolder(path string, handler engines.FileHandler) error {
	files, err := r.metaService.ListFolder(path)
	if err != nil {
		return err
	}

	// TODO: Consider some level of parallelism, but not too many files in parallel
	for _, p := range files {
		f, err := r.metaService.GetArtifact(p)
		if err != nil {
			return err
		}
		if handler(p, f) != nil {
			return engines.ErrHandlerInterrupt
		}
	}

	return nil
}

func (r *resultSet) Dispose() error {
	r.vm.Kill()
	<-r.vm.Done
	return nil
}
