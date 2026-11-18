//go:build !windows

package commands

import (
	"archive/tar"
	"syscall"
)

func mkfifoEntry(hdr *tar.Header, path string) error {
	return syscall.Mkfifo(path, uint32(hdr.Mode&07777))
}
