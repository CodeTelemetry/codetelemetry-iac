//go:build unix

package provider

import (
	"os"
	"syscall"
)

func contextFileOwned(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Getuid())
}
