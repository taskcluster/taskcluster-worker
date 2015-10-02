

type interface Engine {
    // TODO: Figure out how to report async errors, abort and differ between internal error
    // and malformed-payload
    // TODO: Figure out how to configure cache interaction
    AttachCache(source string, string target, readOnly bool) err
    AttachProxy(name string, handler func(ResponseWriter, *Request)) err
    AttachService(image string, command string[], env) err
    Start(command string[], env map[string]string) bool, err
    StdinPipe() io.WriteCloser, err
    StdoutPipe() io.ReadCloser, err
    StderrPipe() io.ReadCloser, err
    NewExec() Exec
    ArchiveFolder(path) <-chan(string, io.ReadCloser)
    ArchiveFile(path) string, io.ReadCloser
    Archive() io.ReadCloser
    Abort()
}

