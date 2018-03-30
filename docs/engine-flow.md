---
title: Engine Life-Cycle
---

Engine Life-Cycle
=================
An engine is always created from an `EngineProvider`. The `EngineProvider` may
specify a JSON schema for the config options it accepts, and when instantiated
it will be passed configuration options and various runtime environment
resources such as log and garbage collector.

An engine is usually only instantiated once during the startup process.
For testing purposes it is encouraged that engines be reentrant, through
this requirement can be avoided when tests are written.

When an `Engine` isn't needed anymore and all of sandbox builders, sandboxes and
result-sets created from it have been disposed, it can be disposed by invocation
of `Dispose()`. This method must release and cleanup all resources used by the
engine. Again, this is mainly useful for testing purposes, but it also
encourages tracking of resources which helps prevent leaks.

The general flow of how the `Engine` abstractions are called when a task is
processed is outlined in the diagram below:
![engine-flow](./engine-flow.svg)


Building a Sandbox
------------------
Once an `Engine` has been instantiated the worker will claim tasks. Depending on
configuration and capabilities of the engine, the worker may try to run multiple
tasks at once. Engines can specify concurrency limitations as necessary, if
concurrent sandboxes aren't supported.

To build a sandbox the worker will call `Engine.NewSandboxBuilder` passing in
payload matching the JSON schema specified by the engine, and a `TaskContext`
object. The `TaskContext` object implements `context.Context` from golang
standard library, if it is canceled any long running process related to the
sandbox builder, sandbox or result-set derived from the
`Engine.NewSandboxBuilder()` call should be aborted. Hence, the sandbox builder
should keep a reference to the `TaskContext` as it will be needed later.

The `SandboxBuilder` should be thread-safe as plugins may concurrently call
all the methods on it, except `SandboxBuilder.StartSandbox()` which will called
by the worker when plugins are done customizing the sandbox.
The worker may also choose to call `SandboxBuilder.Discard()` instead of
`SandboxBuilder.StartSandbox()`, in-order to discard the sandbox builder.

Both starting the sandbox and discarding the sandbox builder invalidates the
`SandboxBuilder` object, either releasing resources or passing the ownership
on to the `Sandbox` object. Notice that if `TaskContext` is canceled during the
`SandboxBuilder.StartSandbox()` then the sandbox builder may choose to either
return a `Sandbox` object, or return the `context.Canceled` error. As a race is
unavoidable either behavior is permitted, as long as resources are either
released or transferred to the `Sandbox` object.


Waiting for a Sandbox
---------------------
Once a `Sandbox` object has been created from the `SandboxBuilder` the task
is running. As with `SandboxBuilder` all methods must be thread-safe, as they
may be used by multiple plugins interacting with the task.

The `Sandbox` object offers methods for creating interactive shells
and displays through VNC connections, these methods may return
`engines.ErrFeatureNotSupported` if not implemented. If implementation of these
features are fragile, implementors should strive to return
`runtime.ErrNonFatalInternalError`, if the error isn't an indicator of an
unhealthy system.

The resources held by a `Sandbox` can be released by `Sandbox.Abort()` or
transferred to a `ResultSet` by `Sandbox.WaitForResult()`. Implementors can
implement `Sandbox.Abort()` as a call to `Sandbox.Kill()` which causes
`Sandbox.WaitForResult()` to return a `ResultSet` with `Success()` being `false`.

When using a `Sandbox` consumers should be aware that there is an inherent race
between `Sandbox.WaitForResult()` and `Sandbox.Abort()`, if task execution
terminates before `Sandbox.Abort`() is called, the `Sandbox` may refuse to abort
and instead require the consumer calls `Sandbox.WaitForResult()` to obtain the
result-set.

When a `Sandbox` is resolved, that is when either `Sandbox.Abort()` or
`Sandbox.WaitForResult()` returns, the resolution is consistent, and any further
call to either `Sandbox.Abort()` or `Sandbox.WaitForResult()` must yield a
consistent result. Indeed multiple calls to `Sandbox.WaitForResult()` should
return the same `ResultSet`.


Extracting Results
------------------
When task execution is finished the sandbox returns a `ResultSet` from
`Sandbox.WaitForResult()`, this invalidates the `Sandbox` object and frees all
resources held by the sandbox or transfers these resources to the `ResultSet`.

The `ResultSet` object offers methods for extracting results from the task
execution. Notably `ResultSet.Success()` which returns if the execution was
successful, typically, equivalent to exit-code zero. As with the sandbox all
methods must be thread-safe, as they may be called by multiple plugins
concurrently.

When extracting files and folders from a `ResultSet` the file-path format is
engine specific. Plugins are expected to obtain these strings from the
task payload. The `ResultSet` should return `runtime.MalformedPayloadError` if
the file-path format is invalid, such that plugins can handle this gracefully.

A `ResultSet` object is disposed by `ResultSet.Dispose()` which should free all
resources held by the `ResultSet`, after this any further calls to the
`ResultSet` will be forbidden.
