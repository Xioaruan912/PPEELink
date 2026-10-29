//go:build !linux

package throughputcore

import (
	"os"
	"os/exec"
	"time"
)

func configureCommand(cmd *exec.Cmd)       { cmd.WaitDelay = time.Second }
func ownedDirectory(info os.FileInfo) bool { return false }
