//go:build !unix

package store

import "os"

// keepOwner is a no-op where files have no Unix owner and group.
func keepOwner(string, os.FileInfo) error { return nil }
