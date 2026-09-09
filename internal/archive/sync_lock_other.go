//go:build !unix

package archive

import (
	"context"
	"errors"
)

func lockSync(context.Context, string) (func(), error) {
	return nil, errors.New("archive sync locking is not supported on this platform")
}
