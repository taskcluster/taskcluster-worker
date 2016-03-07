TaskCluster Worker
==================

<img src="https://tools.taskcluster.net/lib/assets/taskcluster-120.png" />
[![Build Status](https://travis-ci.org/taskcluster/taskcluster-worker.svg?branch=master)](http://travis-ci.org/taskcluster/taskcluster-worker)
[![GoDoc](https://godoc.org/github.com/taskcluster/taskcluster-worker?status.svg)](https://godoc.org/github.com/taskcluster/taskcluster-worker)
[![Coverage Status](https://coveralls.io/repos/taskcluster/taskcluster-worker/badge.svg?branch=master&service=github)](https://coveralls.io/github/taskcluster/taskcluster-worker?branch=master)
[![License](https://img.shields.io/badge/license-MPL%202.0-orange.svg)](http://mozilla.org/MPL/2.0)

A generic worker for task cluster, written in go.


** Warning very EARLY notes**

Place we outline interfaces and patterns for next generation worker.

```
Goal: have all interfaces and central classes defined and documented.




Things we sort of need:
  - garbage collector...
  - state machines... (http://www.timschmelmer.com/2013/08/state-machines-with-state-functions-in.html)
    - basically a state is a function that takes an action and returns a new function (state)



state machine:

type stateFn func(context, event String) stateFn

func init(context, event) stateFn {
  switch event {
    case "loop":
      return init;
  }
}
idea would be that event is some struct that can have different kinds of data
and each state must handle all possible inputs. Inputs could be:
  engine reported that execution finished
  an unhandle error occured
  some file was downloaded (or pre-setup state finished)
  task was cancelled (arrived by pulse message)
  artifacts have been exported
  environment has been cleaned up

various different things that happens. Ideally a task goes from:
  claimed -> loading -> running -> uploading -> cleanup
But sometimes we have cancel, fatal errors (to task), reclaim failed happening
and keeping track of where the state then goes is hard. An explicite state machine
might help. Granted we should keep inputs to a minimum.

We might need two state-machines one for the worker life cycle, with states like:
  booting, claiming, running, shutting down
Again just ideas...


------------------ etherpad
Resources:
https://blog.golang.org/context (maybe there is something smart here - not sure)
Use sync.Mutex


main:
if startFlag == aws:
  host := NewAWSHost() // cloud provider specific function
elseif startFlag == hardware:
  host := NewHWHost() // cloud provider specific function
elseif startFlag == google cloud engine
  host := NewGAEHost() // cloud provider specific function
config := host.LoadConfig()
platform := NewPlatform() // platform specific function
tasks := []Task{};

handler := func(task) {
    tasks = append(tasks, task)
    go func() {
        task.Run(platform)
        task_done<- task
    }
}

for {   // <-- this should probably be a state machine
    count := queue.ClaimTask(slots - len(tasks), this.handler)
    if (len(tasks) == 0 && host.ShouldShutdownNow()) {
        // Shut down
    }
    if count == 0 {
        sleep 5s
    }
    // TODO: handle shutdown from host which lives in a different goroutine
    for {
        select {
            case task <- task_done:
                // Remove task from tasks
            default:
                if len(tasks) >= slots {
                    task <-task_done
                    // Remove task from tasks
                }
                break;
        }
    }
}

QueueService
New(...config)
ClaimTasks(maxTasks, handler) int

Task
New(task, status)
Run(platform) // create Engine and handle logs, updates etc...
Use a function and switch statement for each state

type interface Exec {
    Start(command string[]) bool, err
    StdinPipe() io.WriteCloser, err
    StdoutPipe() io.ReadCloser, err
    StderrPipe() io.ReadCloser, err
    Abort()
}

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

type struct CacheFolder { // not sure about this
    Archive() io.ReadCloser
    Path() string
    Delete() // I can't do anything about errors so just log them, or panic
    Size() uint64
}

type interface Platform {
    NewEngine() Engine
    NewCacheFolder() CacheFolder
}

type interface Host {
    ShouldShutdownNow() bool
    ShuttingDown() <- chan struct{}{}
    LoadConfig()
}

Cache
New(platform)
ExclusiveCache(name string) CacheFolder
SharedCache

```
