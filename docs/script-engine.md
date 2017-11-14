---
title: Script Engine
---

The script-engine allows for declarative tasks. This means that the worker
is configured with a script and a payload schema. The payload is then passed
to the script which is responsible for executing the payload. This is mostly
in cases such as:

 * Security sensitive operations, where tasks shouldn't be allowed to execute
   arbitrary commands.
 * Workers without isolation where only very specific hardcoded actions are
   possible, and payload is used to inject arguments for said actions.

In both cases the main motivation for using script-engine is that it is
undesirable to have tasks execute arbitrary commands. Whether because there is
insufficient isolation or executing security sensitive operations, preventing
tasks from executing arbitrary commands is a good way to avoid tasks causing
havoc on a worker.

Worker Configuration
====================

script-engine is configured with a `command` and `schema` as illustrated below:

```yaml
engine: script
engines:
  script:
    command:
      - /usr/bin/bash
      - /home/jonasfj/worker-script.sh
    schema:
      # type object is required for the root of the schema
      type: object
      properties:
        binaryToSign: {type: 'string', format: 'uri'}
        verbose: {type: 'boolean'}
      required:
        - binaryToSign
plugins:
  disabled:
    # The following plugins are not useful when not executing arbitrary commands
    # as the input is JSON and the script decides what artifacts to output.
    # Similarly, there is no interactive support, when not executing arbitrary
    # commands. Other plugins may or may not be sensible depending on use-case.
    - artifacts
    - env
    - tcproxy
    - interactive
... # other worker configuration keys...
```

Tasks for this worker must now have a `task.payload` that satisfies the given
`schema`. Notice that if other plugins are configured they may accept other
properties on the `task.payload` and the worker will merge such schemas.
An example task for the configuration above could look like:

```js
{
  provisionerId: '...',
  workerType: '...',
  created: '...',
  ... // other top-level keys
  payload: {
    binaryToSign: 'https://example.com/file.tar.gz',
    verbose: true,  // this property is optional
  },
}
```

Script Interface
================

In the configuration example above the command/script configured was
`['/usr/bin/bash', '/home/jonasfj/worker-script.sh']`. This command will
executed for each task with the following interface:

 * **`stdin`** of the command will passed the parts of `task.payload` matching
   configured schema, after which `stdin` is closed.
 * **Environment variables** `TASK_ID` and `RUN_ID` will be accessible to
   the command.
 * **Current working directory** for the command will be a temporary folder
   that will be deleted once the task is completed. This folder will contain
   an `./artifacts/` folder that artifacts should be written to.
   (notice that public artifacts must be written to `./artifacts/public/...`)
 * **`stdout`** of the command will exposed as task log.
 * **`stderr`** of the command will have lines prefixed `[worker:error]` and
   injected into the task log.
 * **Signal `SIGKILL`** will be sent if the command should abort and clean-up
   all sub-processes. Sent if the current task is aborted.
 * **Exit code** will be interpreted as follows:
   * `0`, task completed (success),
   * `1`, task failed,
   * `2`, task exception with reason: `malformed-payload`
     (a message should have been written to stderr),
   * `3`, task exception with reason: `internal-error`, but worker will continue
     (a message should have been written to stderr),
   * `4`, exception with reason: `internal-error`, and worker will terminated gracefully
     (a message should have been written to stderr).
   * Other error codes are reserved.

Naturally, a script like `'/home/jonasfj/worker-script.sh'` should start by
reading `stdin` until `EOF` and then parse the bytes read as UTF-8 encoded JSON.

The command script should generally avoid to resolve tasks with `internal-error`,
however, this can be useful in cases where the inexplicable errors occurs.
From the interface specification above there are two ways to report
`internal-error`, namely exit codes `3` and `4`. The difference between these
is that `4` will stop the worker process from claiming more tasks and terminate.


Robust Deployments
==================

When deploying taskcluster-worker it is often useful to configure the machine
such that if the worker process exits, the machine will be _reset_, and the
worker process restarted. Exactly what it means to _reset_ the machine depends
on the scenario, it could be:
 * Restarting the machine,
 * Destroy virtual machine and start from a snapshot, or,
 * Clean-up some folders and restart taskcluster-worker.

The idea is that `taskcluster-worker` will exit, if it encounters a fatal
internal-error. This could be consistent network errors, exit code `4` from
script, or some other internal error. While not all internal errors are fatal,
the `stoponerror` plugin can be used to terminate on any internal error.

By doing a best-effort reset and restarting `taskcluster-worker` when it exits,
it is possible to make a very robust deployment. Using the `watchdog` and
`maxruntime` plugins it is easy to ensure that `taskcluster-worker` will exit
should get stuck somewhere in the task processing cycle.
