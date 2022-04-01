package dirsync

import (
	"context"
	"github.com/bondhan/sync/pkg/model"
	"io/ioutil"
)

type directorySync struct {
	ctx        context.Context
	fullPath   string
	lists      map[string]model.DirSync
	sortedKeys []string
}

type DirectorySync interface {
	BuildList() error
	GetList() map[string]model.DirSync
}

func New(ctx context.Context, fullPath string) (DirectorySync, error) {
	return &directorySync{
		ctx:      ctx,
		fullPath: fullPath,
	}, nil
}

func (ds *directorySync) BuildList() error {
	files := make(map[string]model.DirSync)

	list, err := ioutil.ReadDir(ds.fullPath)
	if err != nil {
		return err
	}

	for _, l := range list {
		files[l.Name()] = model.DirSync{
			Name:    l.Name(),
			Size:    l.Size(),
			ModTime: l.ModTime(),
		}
	}

	ds.lists = files

	return nil
}

func (ds *directorySync) GetList() map[string]model.DirSync {
	return ds.lists
}
