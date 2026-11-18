//go:build windows

package commands

import "archive/tar"

func mkfifoEntry(hdr *tar.Header, path string) error {
	return nil
}
