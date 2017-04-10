Worker Architecture
===================
It is possible to have many different worker types in taskcluster, indeed we
have multiple worker implementations. This is extremely powerful as we can't
possibly support all future platforms and features with a single code base.
However, just because multiple worker implementations is possible it is still
desirable to reuse code across platforms and aim for consistency in concepts,
features and `task.payload` formats across platforms. `taskcluster-worker` is
an attempt at writing a cross platform worker that support multiple execution
environments and configurable feature-sets.

This does not mean that all thinkable workers can be implemented with
`taskcluster-worker`. The architecture of `taskcluster-worker` aims to abstract
the execution environment and features for any worker where the task specifies
some form of sub-process to be executed. For example, the task could specify a
command to be executed in a virtual machine, docker container or
just an unconfined sub-process. The architecture of `taskcluster-worker` does
not aim to facilitate more declarative tasks, such as sign a binary, where the
`task.payload` specifies the binary to sign, rather than a command.

To support multiple platforms and configurable feature-sets `taskcluster-worker`
have two important abstractions:

 * `engines.Engine`, an abstraction of a sand-boxed execution environment, and,
 * `plugins.Plugin`, an plugin which can implement a complex feature.

This allows for features to be implemented independently of the execution
environment that is used. Additionally, it means that we can add additionally
execution environments without rewriting the worker from scratch.

In the simple terms the worker loads a config file, and sets up some abstracted
resources like:

 * Temporary file storage,
 * A garbage collected cache registry,
 * Logic for exposing public web-hooks (live logs and interactive shells),
 * Log recording, and
 * Life-cycle controls,

Then it instantiates an engine and enters a task processing loop, which in broad
strokes looks somewhat like this:

 1. Claim task
 2. Process task
 3. Repeat

At each step (and many sub-sets) the plugins are called giving them the
opportunity to modify the `SandboxBuilder`, the `Sandbox`, the `ResultSet`, or
to shutdown the worker. Hence, plugins are responsible for implementing all the
interesting features such as:

 * Injection of environment variables,
 * Exposing interactive shells,
 * Extraction of artifacts,
 * Setup proxy services,
 * Control worker life-cycle,
 * ...

By enabling or disabling plugins we can swap-out logic controlling the worker
life-cycle depending on whether we're running in a data-center or on an EC2
spot-node where we have to watch for spot-shutdown events. We can also disable
certain features like interactive shells and live-logs which might be
undesirable in security sensitive environments.

Finally, this architecture serves to decouple the code as much as possible,
making it possible to modify the artifact extraction plugin without touching the
plugin responsible for injecting environment variables.
