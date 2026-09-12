//go:build !linux

package utils

import "os"

func newSeekableMediaFile(extension string, _ int) (*os.File, string, []*os.File, func(), error) {
	return newTemporaryMediaFile(extension)
}
