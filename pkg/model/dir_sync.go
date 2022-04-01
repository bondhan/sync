package model

import "time"

type DirSync struct {
	Name    string
	Size    int64
	ModTime time.Time
}
