---
title: "Design: Engines and Plugins"
order: 20
sequence_diagrams: true
---

## Engines

Engines represent the environment that the worker runs in, such as a native
engine on Windows, a docker engine on linux, a chroot engine on OS X, or even a
mobile device.

At its simplest, an engine is responsible for providing an environment for
executing tasks. It provides features such as setting environment variables,
executing commands, extracting artifacts, mounting caches, etc. You can think
of it as being like a container, that the task runs inside. The specific set of
features that it needs to support is a function of the plugins which are
enabled.

### Engine Lifecycle

The entry point is an
[EngineProvider](https://godoc.org/github.com/taskcluster/taskcluster-worker/engines/extpoints#EngineProvider).
The EngineProvider exposes the factory method `NewEngine(EngineOptions)` which
returns an
[Engine](https://godoc.org/github.com/taskcluster/taskcluster-worker/engines#Engine).
In unit tests, a new engine is created for each test case. When running
normally (not in tests) a single Engine will be created at worker startup, and
will persist until the worker stops/dies.

An Engine in turn provides a factory method `NewSandboxBuilder(SandboxOptions)`
which is the starting point for creating a dedicated task-specific container
for the task. The container starts life as a
[SandboxBuilder](https://godoc.org/github.com/taskcluster/taskcluster-worker/engines#SandboxBuilder),
in which its configuration and setup can be modified.

After this phase, the container can no longer be altered, and it becomes a
[Sandbox](https://godoc.org/github.com/taskcluster/taskcluster-worker/engines#Sandbox).
It holds the same data as the SandboxBuilder, but it exposes different methods,
to make the configuration immutable.

After the task has executed, and a result has been determined, the Sandbox
becomes a
[ResultSet](https://godoc.org/github.com/taskcluster/taskcluster-worker/engines#ResultSet).
Again, it essentially represents the same data as the Sandbox, but now provides
different methods (for example, unlike the Sandbox, the ResultSet cannot be
started or stopped). Using different types provides a guarantee that only valid
methods will be exposed at each lifecycle phase. Finally, the ResultSet is
disposed.

<div class="sequence-diagram-hand">
participant Main
participant EngineProvider
participant Engine
participant SandboxBuilder
participant Sandbox
participant ResultSet

Main           ->  EngineProvider : NewEngine(EngineOptions)
EngineProvider --> Main           : Engine
Main           ->  Engine         : NewSandboxBuilder(SandboxOptions)
Engine         --> Main           : SandboxBuilder
Main           ->  SandboxBuilder : StartSandbox()
SandboxBuilder --> Main           : Sandbox
Main           ->  Sandbox        : WaitForResult()
Sandbox        --> Main           : ResultSet
Main           ->  ResultSet      : Dispose()
</div>

## Plugins

Plugins provide (typically engine-independent) features.

There are plugins for setting environment variables, managing cache folders,
cancelling tasks, serving livelogs, proxying TaskCluster requests, providing
interactive features such as VNC and SSH access, as well as many other things.
And, of course, you can extend the worker with your own custom plugins.

The core of the worker takes care of the claiming tasks,
creating/configuring/calling/destroying engine and plugin instances (called
sandboxes and task plugins, respectively), and handling core features such as
logging, queue polling, etc. These features are all independent of the given
plugins and engines, and therefore the core of the worker should be a stable
base to build highly varied workers on top of, with very different requirements
and constraints.

Plugins can be tested independently of engines (since they are cross-engine)
and engines can be tested independently of the plugins. For features which are
engine-specific, engines may return a _feature not supported_ error. Although
this might sound like an unwanted runtime error, it is not. At worker type
creation time, the set of available features is determined, and a task payload
JSON schema is constructed for the given worker type, and _registered_ with the
Queue. This means the Queue will reject task submissions that violate the
available features of a given worker type, by validating the task definition
against the JSON schema for that worker type.

Therefore it is impossible for workers to claim tasks that require features
that they do not support, and thus _feature not supported_ is not a runtime
error that breaks a task, but rather an error that is interpreted at worker
type creation time to limit the task payload JSON schema that will be
registered with the Queue for that worker type.

---

### Plugin Lifecycle

Much like engines, there is a staged lifecycle for Plugins.
[PluginProvider](https://godoc.org/github.com/taskcluster/taskcluster-worker/plugins/extpoints#PluginProvider)
exposes a method to create a
[Plugin](https://godoc.org/github.com/taskcluster/taskcluster-worker/plugins#Plugin).
In tests a Plugin instance will be created with each test. Otherwise, a single
Plugin will be created at worker startup, and live until the worker stops/dies.

Plugin then provides a method `NewTaskPlugin(TaskPluginOptions)` which returns
a
[TaskPlugin](https://godoc.org/github.com/taskcluster/taskcluster-worker/plugins#TaskPlugin).
This is an instance dedicated to the given task. Unlike the SandboxBuilder ->
Sandbox -> ResultSet counterpart, Plugin does not mutate into other types
during the task lifecycle.

<div class="sequence-diagram-hand">
participant Main
participant PluginProvider
participant Plugin
participant TaskPlugin

Main           ->  PluginProvider : NewPlugin(PluginOptions)
PluginProvider --> Main           : Plugin
Main           ->  Plugin         : NewTaskPlugin(TaskPluginOptions)
Plugin         --> Main           : TaskPlugin
Main           ->  TaskPlugin     : Prepare()
Main           ->  TaskPlugin     : BuildSandbox()
Main           ->  TaskPlugin     : Started()
Main           ->  TaskPlugin     : Stopped()
Main           ->  TaskPlugin     : Finished()
Main           ->  TaskPlugin     : Dispose()
</div>

## Engines and Plugins

Putting this all together, we get the following interactions.

It is important to remember that in a real worker there will be multiple
plugins, and a single engine. TaskPlugins live in their own go routines, and
can operate on the SandboxBuilder/Sandbox/ResultSet in parallel.

Only when all TaskPlugins have completed configuring a SandboxBuilder
(`BuildSandbox()`), will it become a Sandbox.

Only when all TaskPlugins have completed `Stop()`, will `Finished()` be called.

Only when all TaskPlugins have completed `Finished()`, will `Dispose()` be
called against all the TaskPlugins.

This behaviour is implemented via
[WaitGroups](https://golang.org/pkg/sync/#WaitGroup).

<div class="sequence-diagram-hand">
participant Main
participant Plugin
participant TaskPlugin
participant Engine
participant SandboxBuilder
participant Sandbox
participant ResultSet

Main           ->  Plugin         : NewTaskPlugin()
Plugin         --> Main           : TaskPlugin
Main           ->  TaskPlugin     : Prepare()
Main           ->  Engine         : NewSandboxBuilder()
Engine         --> Main           : SandboxBuilder
Main           ->  TaskPlugin     : BuildSandbox()
Note over SandboxBuilder : Sandbox built by ALL plugins
Main           ->  SandboxBuilder : StartSandbox()
SandboxBuilder --> Main           : Sandbox
Main           ->  TaskPlugin     : Started()
Main           ->  Sandbox        : WaitForResult()
Sandbox        --> Main           : ResultSet
Main           ->  TaskPlugin     : Stopped()
Main           ->  TaskPlugin     : Finished()
Main           ->  TaskPlugin     : Dispose()
Main           ->  ResultSet      : Dispose()
</div>


