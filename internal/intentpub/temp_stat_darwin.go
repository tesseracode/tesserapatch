//go:build darwin && !ios

package intentpub

import "syscall"

func sameTempMutationMetadata(first, second syscall.Stat_t) bool {
	return first.Dev == second.Dev &&
		first.Ino == second.Ino &&
		first.Mode == second.Mode &&
		first.Nlink == second.Nlink &&
		first.Size == second.Size &&
		first.Mtimespec.Sec == second.Mtimespec.Sec &&
		first.Mtimespec.Nsec == second.Mtimespec.Nsec &&
		first.Ctimespec.Sec == second.Ctimespec.Sec &&
		first.Ctimespec.Nsec == second.Ctimespec.Nsec
}
