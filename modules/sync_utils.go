package syncutils

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
	dirsyncerr "github.com/bondhan/sync/modules/errors"
	"github.com/bondhan/sync/modules/model"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

func IsDir(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return path, err
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(file)

	fileInfo, err := file.Stat()
	if err != nil {
	}

	if !fileInfo.IsDir() {
		return path, dirsyncerr.ErrNotFile
	}

	return path, nil
}

func CompareFile(src model.DirSync, dst map[string]model.DirSync) (bool, error) {
	d, ok := dst[src.Name]
	if !ok {
		return false, nil
	}

	if d.Size != src.Size {
		return false, nil
	}

	srcSha, err := CalcSHA256(src.Name)
	if err != nil {
		return false, err
	}
	dstSha, err := CalcSHA256(d.Name)
	if err != nil {
		return false, err
	}
	if srcSha != dstSha {
		return false, nil
	}
	return true, nil
}

func ComputeDirSync(i int, ctx context.Context, wg *sync.WaitGroup, done chan struct{},
	sFile chan model.DirSync, errC chan error, dst map[string]model.DirSync, diff map[string]model.DirSync) {
	fmt.Println("goroutine:", i)
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("context canceled")
			return
		case <-done:
			fmt.Println("done received from goroutine", i)
			return
		default:
			for file := range sFile {
				identical, err := CompareFile(file, dst)
				if err != nil {
					errC <- err
					fmt.Println("sending err", errC)

					return
				}
				if !identical {
					diff[file.Name] = file
				}
			}
		}
	}
}

func CalcSHA256(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			fmt.Println("Err:", err)
		}
	}(f)

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return string(h.Sum(nil)), nil
}

func IsEmptyDir(dirName string) (bool, error) {
	f, err := os.Open(dirName)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func walkFiles(done <-chan struct{}, root string) (<-chan inputData, <-chan error) {
	pathdata := make(chan inputData)
	errc := make(chan error, 1)

	go func() {
		defer close(pathdata)
		errc <- filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if !errors.Is(err, fs.ErrPermission) {
					return err
				}
				fmt.Println("Err:", err, path, "will be skipped")
				return nil
			}

			// get the file info
			f, _err := d.Info()
			if _err != nil {
				return err // internal error
			}

			// if it is directory
			if f.IsDir() {
				// and check if empty
				isEmpty, errEmpty := IsEmptyDir(path)
				if errEmpty != nil { // if we found error during checking, blacklist
					if !errors.Is(errEmpty, fs.ErrPermission) {
						return err
					}
					fmt.Println("Err:", errEmpty, path, "will be skipped")
					return nil
				}
				if isEmpty { // skip if empty directory
					fmt.Println(path, "is empty folder, will be skipped")
					return nil
				}
			}

			id := inputData{path, f.Size(), d.IsDir()}
			select {
			case pathdata <- id:

			case <-done:
				fmt.Println("done in walkFiles")
				return errors.New("walk canceled")
			}
			return nil
		})
	}()

	return pathdata, errc
}

// A result is the product of reading and summing a file using MD5.
type result struct {
	sourcePath string
	destPath   string
	sum        [md5.Size]byte
	err        error
}

// A result is the product of reading and summing a file using MD5.
type inputData struct {
	path  string
	size  int64
	isDir bool
}

func digester(done <-chan struct{}, paths <-chan inputData, c chan<- result) {
	for fInput := range paths { // HLpaths
		if fInput.isDir {
			//check if empty
			//check if destination has it
			//if not create dir in destination
		} else {
			//check if destination exist
			//if not exist return the file to be copy
			//if exist calculate the md5 destination and source
			//if not equal then return the file to be copy

			data, err := ioutil.ReadFile(fInput.path)
			select {
			case c <- result{fInput.path, fInput.path, md5.Sum(data), err}:
			case <-done:
				return
			}
		}
	}
}
func DoSync(ctx context.Context, srcRoot string, dstRoot string) (map[string][md5.Size]byte, error) {
	done := make(chan struct{})
	defer close(done)

	// level 1, walk the source file system
	pathdata, errc := walkFiles(done, srcRoot)

	c := make(chan result)
	var wg sync.WaitGroup
	const numDigesters = 20
	wg.Add(numDigesters)
	for i := 0; i < numDigesters; i++ {
		go func() {
			digester(done, pathdata, c) // HLc
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(c)
	}()

	m := make(map[string][md5.Size]byte)
	for r := range c {
		if r.err != nil {
			fmt.Println("Err:", r.err)
			continue
		}
		m[r.sourcePath] = r.sum
	}

	// Check whether the Walk failed.
	if err := <-errc; err != nil {
		fmt.Println("walkFiles err:", err)
		return nil, err
	}
	return m, nil

	// level 2, check if:
	// 	- is empty directory then ignore
	//   - check if file exist and identical with destination

	// level 3, copy file to destination

	// Return err
	return m, nil
}
