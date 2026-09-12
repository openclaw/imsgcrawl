//go:build !darwin

package sqlitesnapshot

import "os"

func cloneFile(_, _ string, _ os.FileInfo) (bool, error) {
	return false, nil
}
