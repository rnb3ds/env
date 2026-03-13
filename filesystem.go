package env

import (
	"io"
	"os"
)

// FileSystem defines the interface for file system operations.
// This interface allows for dependency injection, making code more testable
// by enabling mock implementations.
type FileSystem interface {
	// Open opens a file for reading.
	Open(name string) (File, error)

	// OpenFile opens a file with the specified flags and permissions.
	OpenFile(name string, flag int, perm os.FileMode) (File, error)

	// Stat returns file information.
	Stat(name string) (os.FileInfo, error)

	// MkdirAll creates a directory and any necessary parents.
	MkdirAll(path string, perm os.FileMode) error

	// Remove removes a file.
	Remove(name string) error

	// Rename renames a file.
	Rename(oldpath, newpath string) error

	// Getenv retrieves the value of the environment variable named by the key.
	Getenv(key string) string

	// Setenv sets the value of the environment variable named by the key.
	Setenv(key, value string) error

	// Unsetenv unsets the environment variable named by the key.
	Unsetenv(key string) error

	// LookupEnv retrieves the value of the environment variable named by the key.
	LookupEnv(key string) (string, bool)
}

// File defines the interface for file operations.
// It combines common io interfaces with file-specific operations.
type File interface {
	io.Reader
	io.Writer
	io.Closer
	Stat() (os.FileInfo, error)
	Sync() error
}

// OSFileSystem implements FileSystem using the real operating system.
// OSFileSystem is safe for concurrent use.
// The zero value is valid and ready to use.
type OSFileSystem struct{}

// Open opens a file for reading using os.Open.
func (OSFileSystem) Open(name string) (File, error) {
	return os.Open(name)
}

// OpenFile opens a file with the specified flags and permissions.
func (OSFileSystem) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return os.OpenFile(name, flag, perm)
}

// Stat returns file information using os.Stat.
func (OSFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// MkdirAll creates a directory and any necessary parents using os.MkdirAll.
func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Remove removes a file using os.Remove.
func (OSFileSystem) Remove(name string) error {
	return os.Remove(name)
}

// Rename renames a file using os.Rename.
func (OSFileSystem) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// Getenv retrieves the value of the environment variable using os.Getenv.
func (OSFileSystem) Getenv(key string) string {
	return os.Getenv(key)
}

// Setenv sets the value of the environment variable using os.Setenv.
func (OSFileSystem) Setenv(key, value string) error {
	return os.Setenv(key, value)
}

// Unsetenv unsets the environment variable using os.Unsetenv.
func (OSFileSystem) Unsetenv(key string) error {
	return os.Unsetenv(key)
}

// LookupEnv retrieves the value of the environment variable using os.LookupEnv.
func (OSFileSystem) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

// DefaultFileSystem is the default file system implementation using the real OS.
var DefaultFileSystem FileSystem = OSFileSystem{}
