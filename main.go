package main

import (
	"context"
	"flag"
	"fmt"
	dsync "github.com/bondhan/sync/modules"
	"github.com/bondhan/sync/modules/errors"
	"os"
	"os/signal"
	"syscall"
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
		return false, dsyncerr.ErrNotDirectory
	}

	return true, nil
}

func main() {
	var src, dest string
	var isVerbose, createEmptyFolder bool
	flag.StringVar(&src, "s", "", "source folder")
	flag.StringVar(&dest, "d", "", "destination folder")
	flag.BoolVar(&isVerbose, "v", false, "verbose")
	flag.BoolVar(&createEmptyFolder, "e", false, "create empty folder")
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

	ds, err := dsync.New(ctx, src, dest, dsync.WithVerbose(isVerbose), dsync.WithCreateEmptyFolder(createEmptyFolder))
	checkErr(err)

	// Setting up a channel to capture system signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL)

	go func() {
		<-stop
		cancel()
	}()

	err = ds.DoSync(ctx)
	checkErr(err)

	fmt.Println("Total process:", ds.GetTotal())
}
