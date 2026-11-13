package admin

import "golang.org/x/sys/unix"

// FREAD from <sys/fcntl.h>: TIOCFLUSH takes which queue to flush, and this is the input one.
const fread = 1

func discardInput(fd int) error {
	return unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, fread)
}
