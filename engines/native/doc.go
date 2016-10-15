// Package nativeengine provides an engine with minimal sandboxing relying on
// per-task user accounts, temporary folders and process isolation.
//
// Platform specific methods such as run sub-process under a difference user,
// add/remove users and management of user permissions are all prefix nativeXXX
// and implemented in native_$GOOS.go.
package nativeengine
