package system

type process struct {
}

func (system) NewProcess(options ProcessOptions) Process {
	return nil
}

func (p *process) Wait() bool {
	// TODO: Wait for process to terminate
	return false
}

func (p *process) Kill() error {
	return nil
}
