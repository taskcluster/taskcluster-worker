package system

// Group abtraction for a system user-group.
type Group interface {
	// Grant read/write/execute rights to folder recursively, such that it applies
	// all future files and folder created in folder.
	GrantReadWriteRecursive(folder string) error
	// Grant read/execute rights to folder recursively, such that it applies to
	// all future files and folder created in the folder. 
	GrantReadRecursive(folder string) error
	// Remove group
	Remove() error
}

SetFolderOwner(folder string, owner User) error
