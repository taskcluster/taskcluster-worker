// Package system implements cross-platform abstractions for user-management
// access-control and sub-process execution geared at executing sub-process with
// best-effort sandboxing.
//
// The system package provides the following platform specific types and
// methods.
//      system.User
//      system.User.Remove()
//      system.Group
//      system.Process
//      system.Process.Wait() bool
//      system.Process.Kill()
//      system.SetSize(columns, rows uint16) error
//     	system.CreateUser(homeFolder string, groups []*Group) (*User, error)
//      system.FindGroup(name string) (*Group, error)
//     	system.StartProcess(options ProcessOptions) (*Process, error)
//     	system.KillByOwner(user *User) error
package system

// TODO: Implement the following methods to support cache folder.
//      system.Group.Remove()
//      system.CreateGroup() (*Group, error)
//      system.Link(target, source string) error
//      system.Unlink(target string) error
//      system.SetRecursiveReadWriteRecursive(folder string, group *Group) error
//      system.SetRecursiveReadOnlyAccess(folder string, group *Group) error

import "github.com/taskcluster/taskcluster-worker/runtime"

var debug = runtime.Debug("system")
