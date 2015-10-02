
// All platform implementations must implement the NewPlaform() method
// It must return an implementation of the Platform interface.
// All platform implementations are located in a platform specific folder:
//   platforms/<name>/
// And they must all have a build contraint <name>, so we only ever build
// one platform at the time.

type interface Platform {
	NewEngine() Engine
	NewCacheFolder() CacheFolder
}

