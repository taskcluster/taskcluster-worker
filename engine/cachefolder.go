package engine

// CacheFolder that we can modify and mount on Execution engines.
//
// Note, that engine implementations are not responsible for tracking the
// CacheFolder, deletion and/or if it's mounted on more than one Execution at
// the same time.
type CacheFolder interface {
	// Dispose deletes all resources used by the cacheFolder
	Dispose() error
}
