package admin

import "golang.org/x/sys/unix"

func discardInput(fd int) error {
	return unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIFLUSH)
}
