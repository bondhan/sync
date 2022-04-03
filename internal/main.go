package main

import (
	"flag"
	"fmt"
	"io/fs"
	"path/filepath"
)

func visit(path string, di fs.DirEntry, err error) error {
	fmt.Printf("Visited: %s\n", path)
	return nil
}

func main() {
	flag.Parse()
	root := flag.Arg(0)
	err := filepath.WalkDir(root, visit)
	fmt.Printf("filepath.WalkDir() returned %v\n", err)
}
