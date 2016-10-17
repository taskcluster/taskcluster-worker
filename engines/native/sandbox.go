package nativeengine

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/engines"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
)

type sandbox struct {
	engines.SandboxBase
	engine    *engine
	context   *runtime.TaskContext
	log       *logrus.Entry
	folder    runtime.TemporaryFolder
	resolve   atomics.Once // Protecting resultSet, resultErr and abortErr
	resultSet *resultSet
	resultErr error
	abortErr  error
}

func newSandbox(b *sandboxBuilder) (*sandbox, error) {
	// Create temporary folder for the task
	folder, err := b.engine.environment.TemporaryStorage.NewFolder()
	if err != nil {
		b.log.Error("Failed to create temporary folder: ", err)
		return nil, fmt.Errorf("Failed to temporary folder, error: %s", err)
	}

	s := &sandbox{
		engine:  b.engine,
		context: b.context,
		log:     b.log,
		folder:  folder,
	}

	go s.waitForTermination()

	return s, nil
}

func (s *sandbox) waitForTermination() {
	// TODO: Wait for result

	s.resolve.Do(func() {
		s.resultSet = &resultSet{
			engine:  s.engine,
			context: s.context,
			log:     s.log,
		}
		s.abortErr = engines.ErrSandboxTerminated
	})
}

func (s *sandbox) WaitForResult() (engines.ResultSet, error) {
	// Wait for result and terminate
	s.resolve.Wait()
	return s.resultSet, s.resultErr
}

func (s *sandbox) Abort() error {
	s.resolve.Do(func() {
		// TODO: Abort execution

		s.resultErr = engines.ErrSandboxAborted
	})
	s.resolve.Wait()
	return s.abortErr
}
