//go:build !unix

package provider

import "os"

// Windows protects the per-user directory through ACLs rather than POSIX mode bits.
func contextFileOwned(_ os.FileInfo) bool { return true }
