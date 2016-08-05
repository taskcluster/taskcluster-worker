package ioext

import "os"

// IsPlainFile returns an true if filePath is a plain file, not a directory,
// symlink, device, etc.
func IsPlainFile(filePath string) bool {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return false
	}
	return IsPlainFileInfo(fileInfo)
}

// IsPlainFileInfo returns true, if info is for a plain file, not a dictionary,
// symlink, device, etc.
func IsPlainFileInfo(info os.FileInfo) bool {
	return info.Mode()&(os.ModeDir|os.ModeSymlink|os.ModeNamedPipe|os.ModeSocket|os.ModeDevice) == 0
}

// IsFileLessThan returns true if filePath is a file less than maxSize
func IsFileLessThan(filePath string, maxSize int64) bool {
	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return false
	}
	return fileInfo.Size() < maxSize
}
