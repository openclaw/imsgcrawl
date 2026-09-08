//go:build !unix && !windows

package archive

import "os"

func archiveHardlinked(_ string, _ os.FileInfo) (bool, error) {
	return false, nil
}
