// Package plugins defines interfaces to be implemented by feature plugins.
//
// A plugin is an object that gets called for each step in the task execution
// flow. At each step the plugin will get access to engine and runtime object,
// such as the SandboxBuilder, the Sandbox, the SandboxContext, etc.
// A plugin is then supposed to implement a specific feature using these object,
// this could be live logging, artifact uploading, attachment of proxies,
// mouting of caches, archival of caches, and many other things.
//
// A plugin does not get any plugin specific task.payload space, instead the
// options required by a plugin must be part of the task.payload. Hence, all
// plugins must be available on all platforms. To faciliate this plugins should
// not use platform specific APIs, instead they should rely on interface offered
// by the engine objects to do so. And fail gracefully if an engine method is
// not supported in a given configuration and returns ErrFeatureNotSupported.
//
// When a plugin encounters an unsupported feature that it needs, it may either
// return a MalformedPayloadError, or simply ignore the error and workaround it.
//
// Plugin packages should provide a method:
//   NewXXXPluginFactory(engine.Engine,*runtime.EngineContext) PluginFactory
// and have it registered in pluginmanager.go
//
// At high-level PluginFactory stores the global state owned by the plugin, and
// Plugin stores the task-specific state owned by the plugin. A new Plugin
// instance will be created for each task.
//
// In summery, plugins are not really "plugins", they are merely abstractions
// that allows us to implement features in complete isolation. Maybe they will
// become more flexible in the future, but there is no need to design for this
// at this point.
package plugins
