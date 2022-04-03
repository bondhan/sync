package dirsyncerr

import "errors"

var (
	ErrNotFile = errors.New("not a directory")
)
