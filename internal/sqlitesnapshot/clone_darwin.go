package sqlitesnapshot

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func cloneFile(source, dest string, expected os.FileInfo) (bool, error) {
	file, err := os.Open(source)
	if err != nil {
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !sameState(expected, info) {
		return false, os.ErrNotExist
	}
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return false, err
	}
	// Flagged files can produce immutable clones that cannot be recovered or
	// cleaned up. Keep the portable byte copy for these source files.
	if stat.Flags != 0 {
		return false, nil
	}
	err = unix.Fclonefileat(int(file.Fd()), unix.AT_FDCWD, dest, unix.CLONE_NOOWNERCOPY)
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EXDEV) {
		return false, nil // non-APFS or a temporary directory on another volume
	}
	if err != nil {
		return false, err
	}
	// Only the private clone is changed. Do not inherit source permissions or
	// flags that could prevent SQLite recovery in the temporary directory.
	if err := unix.Chflags(dest, 0); err != nil {
		return false, err
	}
	if err := os.Chmod(dest, 0o600); err != nil {
		return false, err
	}
	return true, nil
}
