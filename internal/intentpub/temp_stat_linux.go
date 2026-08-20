//go:build linux && !android

package intentpub

import "syscall"

func sameTempMutationMetadata(first, second syscall.Stat_t) bool {
	return first.Dev == second.Dev &&
		first.Ino == second.Ino &&
		first.Mode == second.Mode &&
		first.Nlink == second.Nlink &&
		first.Size == second.Size &&
		first.Mtim.Sec == second.Mtim.Sec &&
		first.Mtim.Nsec == second.Mtim.Nsec &&
		first.Ctim.Sec == second.Ctim.Sec &&
		first.Ctim.Nsec == second.Ctim.Nsec
}
