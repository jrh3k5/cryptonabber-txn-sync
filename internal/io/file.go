package io

import (
	"fmt"
	"os"
)

// FileExists checks to see if a file exists at the given path.
func FileExists(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err == nil:
		return true, nil
	default:
		return false, fmt.Errorf("failed to check for existence of file at path '%s': %w", filePath, err)
	}
}
