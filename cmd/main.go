package main

import (
	"context"
	"flag"
	"fmt"
	syncutils "github.com/bondhan/sync/modules"
	"os"
	"sort"
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

	srcRoot, err := syncutils.IsDir(src)
	checkErr(err)

	dstRoot, err := syncutils.IsDir(dest)
	checkErr(err)

	m, err := syncutils.DoSync(ctx, srcRoot, dstRoot)
	checkErr(err)

	var paths []string
	for path := range m {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Printf("%x  %s\n", m[path], path)
	}
}
