package dsyncerr

import "errors"

var (
	ErrNotDirectory          = errors.New("not a directory")
	ErrSameSourceDestination = errors.New("source must not be the same with destination")
)
