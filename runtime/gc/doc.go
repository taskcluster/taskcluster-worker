// Package gc contains the GarbageCollector which allows cacheable resources
// to register themselves for diposal when we run low on resources.
//
// The idea is that cacheable resources like cache folders, docker images,
// downloaded artifacts. Any resource that can be recreated if needed, but
// that we would like to keep around for as long as possible, assuming it
// doesn't affect system resources.
//
// To do this we need to be able to dispose resources when the system runs low
// on disk space or memory. This easily gets complicated, especially as we may
// want to optimize by diposing least-recently-used first. So to simplify it
// cacheable resources should implement the Disposable interface and be
// registered with the GarbageCollector, so it prioritize disposal, even when
// we have different types of cacheable resources.
package gc
