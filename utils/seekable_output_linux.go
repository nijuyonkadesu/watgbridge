//go:build linux

package utils

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func newSeekableMediaFile(extension string, childFileDescriptor int) (*os.File, string, []*os.File, func(), error) {
	fd, err := unix.MemfdCreate("watgbridge-media-"+extension, unix.MFD_CLOEXEC)
	if err == nil {
		outputFile := os.NewFile(uintptr(fd), "watgbridge-media-"+extension)
		return outputFile, fmt.Sprintf("/proc/self/fd/%d", childFileDescriptor), []*os.File{outputFile}, func() {
			_ = outputFile.Close()
		}, nil
	}

	return newTemporaryMediaFile(extension)
}
