// Package config provides configuration loading logic. Similar to how engines
// and plugins work each sub-package of implements a TransformationProvider
// interface, which provides a method to transform configuration values.
//
// The top-level config file for taskcluster-worker has the following form:
//   transforms:
//    - packet
//    - env
//    - secrets
//   config:
//    ... // options to be transformed
//
// In the example above configuration options from `config` will be transformed
// by the packet, env, and secrets TransformationProviders, in the order given.
//
// After all configured TransformationProviders have run
// the configuration object constructed will be validated against the config
// schema required by the 'worker' package.
//
// A TransformationProvider gets the configuration object and can do any
// transformations it desires. For example the "env" transformation will replace
// any object on the form {$env: VAR} with the value of the environment variable
// VAR.
package config
