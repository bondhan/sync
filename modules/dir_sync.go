package dsync

import (
	"context"
	"crypto/md5" //nolint:gosec
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type DirSync struct {
	ctx        context.Context
	SrcRoot    string
	DstRoot    string
	AbsSrcRoot string
	AbsDstRoot string
	TotalFiles int64
}

type DirSyncImpl interface {
	IsEmptyDir(dirName string) (bool, error)
	MakeDirIfEmpty(dirName string) error
	IsFileExist(filename string) bool
	GetFileSize(fileName string) (int64, error)
	IsFileReadable(fileName string) (bool, error)
	IsFileWriteable(fileName string) (bool, error)
	DoSync(ctx context.Context) error
}

func New(ctx context.Context, srcRoot string, dstRoot string) (DirSyncImpl, error) {
	absSrc, err := filepath.Abs(srcRoot)
	if err != nil {
		return nil, err
	}

	absDst, err := filepath.Abs(dstRoot)
	if err != nil {
		return nil, err
	}

	return &DirSync{
		ctx:        ctx,
		SrcRoot:    srcRoot,
		DstRoot:    dstRoot,
		AbsSrcRoot: absSrc,
		AbsDstRoot: absDst,
		TotalFiles: 0,
	}, nil
}

func (ds *DirSync) IsEmptyDir(dirName string) (bool, error) {
	file, err := os.Open(dirName)
	if err != nil {
		return false, err
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(file)

	_, err = file.Readdirnames(1) // Or f.Readdir(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}

func (ds *DirSync) MakeDirIfEmpty(dirName string) error {
	// check if destination folder exist
	_, err := os.Stat(dirName)
	if os.IsNotExist(err) {
		// if not exist then create it
		err = os.Mkdir(dirName, 0755)
		if err != nil && os.IsNotExist(err) {
			fmt.Println("dirname:", dirName)
			fmt.Println("Err:", err) // we log it and pass the error
			return err
		}
		fmt.Println(dirName, "succesfully created") // we log it and pass the error
	}
	return nil
}

func (ds *DirSync) IsFileExist(filename string) bool {
	// check if destination folder exist
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

//nolint:funlen
func (ds *DirSync) walkFiles(done <-chan struct{}, srcRoot string, dstRoot string) (<-chan inputData, <-chan error) {
	pathData := make(chan inputData)
	errC := make(chan error, 1)

	go func() {
		defer close(pathData)
		errC <- filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
			if path == srcRoot {
				return nil
			}

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
				fmt.Println("Internal Err:", err, path, "will be skipped")
				return _err // internal error
			}

			// if it is directory
			if f.IsDir() {
				// and check if empty
				isEmpty, errEmpty := ds.IsEmptyDir(path)
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

			readable, err := ds.IsFileReadable(path)
			if err != nil {
				fmt.Println("Readable error:", err, path, "will be skipped")
				return err // internal error
			}

			if !readable {
				fmt.Println(path, "cannot be read, will be skipped")
				return nil
			}

			// prepare the destination path
			dstPath := fmt.Sprintf("%s%s", dstRoot, strings.TrimPrefix(path, srcRoot))

			id := inputData{path, dstPath, f.Size(), d.IsDir()}
			select {
			case pathData <- id:

			case <-done:
				fmt.Println("done in walkFiles")
				return errors.New("walk canceled")
			}
			return nil
		})
	}()

	return pathData, errC
}

// A result is the product of reading and summing a file using MD5.
type result struct {
	sourcePath string
	destPath   string
	err        error
}

// A result is the product of reading and summing a file using MD5.
type inputData struct {
	srcPath string
	dstPath string
	srcSize int64
	isDir   bool
}

func (ds *DirSync) GetFileSize(fileName string) (int64, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return 0, err
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(file)

	fInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}
	return fInfo.Size(), nil
}

func (ds *DirSync) IsFileReadable(fileName string) (bool, error) {
	file, err := os.OpenFile(fileName, os.O_RDONLY, 0666)
	if err != nil {
		if os.IsPermission(err) {
			return false, nil
		}
		return false, err
	}
	err = file.Close()
	if err != nil {
		fmt.Println(err)
	}

	return true, nil
}

func (ds *DirSync) IsFileWriteable(fileName string) (bool, error) {
	file, err := os.OpenFile(fileName, os.O_WRONLY, 0666)
	if err != nil {
		if os.IsPermission(err) {
			return false, nil
		}
		return false, err
	}
	err = file.Close()
	if err != nil {
		fmt.Println(err)
	}
	return true, nil
}

func (ds *DirSync) checker(done <-chan struct{}, paths <-chan inputData, c chan<- result) {
	for fInput := range paths {
		// fmt.Println(fInput.srcPath, "-", fInput.dstPath)
		var err error
		if fInput.isDir {
			err = ds.MakeDirIfEmpty(fInput.dstPath)
			if err == nil {
				continue
			}
		} else if ds.IsFileExist(fInput.dstPath) {
			// check the srcSize
			dstSize, err := ds.GetFileSize(fInput.dstPath)
			if err == nil {
				if dstSize == fInput.srcSize {
					dataSrc, err := ioutil.ReadFile(fInput.srcPath)
					if err != nil {
						// skip the file
						fmt.Println("ioutil.ReadFile(fInput.srcPath) err:", err)
						continue
					}
					dataDst, err := ioutil.ReadFile(fInput.dstPath)
					if err != nil {
						// skip the file
						fmt.Println("ioutil.ReadFile(fInput.dstPath) err:", err)
						continue
					}
					if md5.Sum(dataSrc) == md5.Sum(dataDst) { //nolint:gosec
						// skip the file as identical
						continue
					}
				}
			} else {
				fmt.Println("else:", err)
			}
		}
		select {
		// list of files need to be copied
		case c <- result{fInput.srcPath, fInput.dstPath, err}:
		case <-done:
			return
		}
	}
}
func (ds *DirSync) DoSync(ctx context.Context) error {
	done := make(chan struct{})
	defer close(done)

	// level 1, walk the source directory recursively
	pathdata, errc := ds.walkFiles(done, ds.AbsSrcRoot, ds.AbsDstRoot)

	c := make(chan result)
	var wg sync.WaitGroup

	// number of check workers to validate if need to do copy or no
	const numCheckers = 20
	wg.Add(numCheckers)
	for i := 0; i < numCheckers; i++ {
		go func() {
			ds.checker(done, pathdata, c) // HLc
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(c)
	}()

	count := 0
	for r := range c {
		count++
		if r.err != nil {
			fmt.Println("Err r.err:", r.err)
			continue
		}

		input, err := ioutil.ReadFile(r.sourcePath)
		if err != nil {
			fmt.Println("Error Read input:", err)
			return err
		}

		err = ioutil.WriteFile(r.destPath, input, 0755) //nolint:gosec
		if err != nil {
			fmt.Println("Error creating", r.destPath, "Err:", err)
			return err
		}
	}

	// Check whether the Walk failed.
	if err := <-errc; err != nil {
		fmt.Println("walkFiles err:", err)
		return err
	}

	// Return err
	return nil
}
