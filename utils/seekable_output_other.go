//go:build !linux

package utils

import "os"

func newSeekableAnimationOutput() (*os.File, string, []*os.File, func(), error) {
	return newTemporaryAnimationOutput()
}
