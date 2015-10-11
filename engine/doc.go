// Package engine specfies the interfaces that each engine must implement.
//
// This is all rooted at the NewEngine method which returns and instance of
// the Engine interface. Each engine implementation must provide a
// NewXXXEngine method and have listed in the NewEngine method so the engine
// can be initialized. The NewXXXEngine method may return nil if the engine isn't
// available given the current build constraints, GOOS or GOARCH.
//
// Notice that many of the features specified are optional, and it's often
// possible to return an ErrFeatureNotSupported error rather than implementing
// the feature.
package engine
