package util

import (
	"os"
	"path/filepath"
)

// WalkFilepath lists the files with the given extension under path. Entries
// that cannot be read, including a missing path, are skipped.
func WalkFilepath(path string, fileExt string) ([]string, error) {
	validPaths := []string{}
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			if filepath.Ext(path) == fileExt {
				validPaths = append(validPaths, path)
			}
		}
		return nil
	})

	return validPaths, err
}
