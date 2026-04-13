// Package maybe provides a way to atomically create or replace a file, if
// technically possible.
package maybe

import (
	"os"

	"github.com/google/renameio"
)

// WriteFile mirrors [os.WriteFile]. On Unix it uses [renameio.WriteFile] to
// create or replace an existing file with the same name atomically. With the
// TierSum renameio fork, the same implementation is used on Windows as well.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return renameio.WriteFile(filename, data, perm)
}
