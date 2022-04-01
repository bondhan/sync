package main

import (
	"context"
	"flag"
	"fmt"
	dirsync "github.com/bondhan/sync/pkg"
	"os"
	"sync"
)

func checkErr(err error) {
	if err != nil {
		fmt.Println("Err:", err)
		os.Exit(1)
	}
}

func main() {
	var src, dest string
	flag.StringVar(&src, "s", "", "source folder")
	flag.StringVar(&dest, "d", "", "destination folder")
	flag.Parse()

	if dest == "" || src == "" {
		fmt.Println("Usage: sync [-ds], where:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srcRoot, err := dirsync.IsDir(src)
	checkErr(err)

	destRoot, err := dirsync.IsDir(dest)
	checkErr(err)

	srcDirSync, err := dirsync.New(ctx, srcRoot)
	checkErr(err)

	destDirSync, err := dirsync.New(ctx, destRoot)
	checkErr(err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func(ds dirsync.DirectorySync) {
		err = ds.BuildList()
		checkErr(err)
		wg.Done()
	}(srcDirSync)

	wg.Add(1)
	go func(ds dirsync.DirectorySync) {
		err = ds.BuildList()
		checkErr(err)
		wg.Done()
	}(destDirSync)

	wg.Wait()

	res, err := dirsync.ComputeDiff(srcDirSync.GetList(), destDirSync.GetList())
	checkErr(err)

	dirsync.Print(res)
}
