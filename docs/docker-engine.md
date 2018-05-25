---
title: Docker Engine
---

The docker-engine runs tasks in a per-task docker container, providing lightweight task isolation.

Worker Configuration
--------------------

The examples folder contains a `docker-config.yml` illustrating how to configure the worker
when using the docker-engine.

Currently, it makes sense to enable the following plugins, when using docker-engine.

 * `artifacts`,
 * `env`,
 * `livelog`,
 * `logprefix`,
 * `tcproxy`,
 * `cache`,
 * `maxruntime`,
 * `success`,
 * `watchdog`,
 * `relengapi`,

Example Payload Schema
----------------------

The payload schemas changes based on how the worker is configured, in some future
a worker manager will expose per-workerType documentation. But if configured like
the `examples/docker-config.yml` the payload schema will look like:

<div data-render-schema='docker-engine-payload-schema.json'></div>
