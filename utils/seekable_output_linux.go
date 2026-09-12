//go:build linux

package utils

import (
	"os"

	"golang.org/x/sys/unix"
)

func newSeekableAnimationOutput() (*os.File, string, []*os.File, func(), error) {
	fd, err := unix.MemfdCreate("watgbridge-sticker-webm", unix.MFD_CLOEXEC)
	if err == nil {
		outputFile := os.NewFile(uintptr(fd), "watgbridge-sticker-webm")
		return outputFile, "/proc/self/fd/3", []*os.File{outputFile}, func() {
			_ = outputFile.Close()
		}, nil
	}

	return newTemporaryAnimationOutput()
}
