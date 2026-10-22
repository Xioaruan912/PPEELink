//go:build !windows

package xraycore

import (
	"os"
	"syscall"
)

func execSelf(path string) error {
	return syscall.Exec(path, os.Args, os.Environ())
}
