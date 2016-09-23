// Package config provides configuration loading logic. Similar to how engines
// and plugins work each sub-package of implements a Provider interface, takes
// options specified using a schema and provides a method to load config.
//
// The top-level config file for taskcluster-worker has the following form:
//   - provider: static
//     options:
//       ... // Options for the static config provider
//   - provider: secrets
//     options:
//       ... // Options for the secrets provider
//
// In the example above configuration options will first be loaded by the
// static config provider, then passed to secrets config provider which will be
// able to overwrite any keys it wants to.
//
// Options given for each provider will be validated against the schema the
// config provider specified. After all configured config providers have run
// the configuration object constructed will be validated against the config
// schema required by the 'worker' package.
//
// Notice, the 'static' config provider takes options that exactly matches the
// schema required by the 'worker' package and sets these options on the
// configuration object.
package config
