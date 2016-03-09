// Package engines specifies the interfaces that each engine must implement.
//
// This is all rooted at the EngineProvider interface specified in the extpoints
// package, where implementors should register their engine.
//
// Consumers of this package should import the extpoints package and use the
// 'EngineProvider.Lookup(name) EngineProvider' function to load the desired
// engine.
//
// Notice that many of the features specified are optional, and it's often
// possible to return an ErrFeatureNotSupported error rather than implementing
// the feature.
package engines
