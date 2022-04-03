package main

import (
	"context"
	"flag"
	"fmt"
	dsync "github.com/bondhan/sync/modules"
	"github.com/bondhan/sync/modules/errors"
	"os"
)

func checkErr(err error) {
	if err != nil {
		fmt.Println("Err:", err)
		os.Exit(1)
	}
}

// isDir will check if path is directory, if
// not will return false and error
func isDir(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(file)

	fileInfo, err := file.Stat()
	if err != nil {
		return false, err
	}

	if !fileInfo.IsDir() {
		return false, dsyncerr.ErrNotFile
	}

	return true, nil
}

func main() {
	var src, dest string
	var isVerbose bool
	flag.StringVar(&src, "s", "", "source folder")
	flag.StringVar(&dest, "d", "", "destination folder")
	flag.BoolVar(&isVerbose, "v", false, "verbose")
	flag.Parse()

	if dest == "" || src == "" {
		fmt.Println("Usage: sync [-ds], where:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := isDir(src)
	checkErr(err)

	_, err = isDir(dest)
	checkErr(err)

	ds, err := dsync.New(ctx, src, dest, dsync.WithVerbose(isVerbose))
	checkErr(err)

	err = ds.DoSync(ctx)
	checkErr(err)
}
