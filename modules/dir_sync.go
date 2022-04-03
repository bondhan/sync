package dsync

import (
	"context"
	"crypto/md5" //nolint:gosec
	"errors"
	"fmt"
	dsyncerr "github.com/bondhan/sync/modules/errors"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const WorkerCount = 20

type result struct {
	sourcePath string
	destPath   string
	err        error
}

type InputData struct {
	srcPath string
	dstPath string
	srcSize int64
	isDir   bool
}

type DirSync struct {
	ctx               context.Context
	SrcRoot           string
	DstRoot           string
	AbsSrcRoot        string
	AbsDstRoot        string
	TotalFiles        int64
	IsVerbose         bool
	CreateEmptyFolder bool
	lock              sync.Mutex
}

type DirSyncImpl interface {
	IsEmptyDir(dirName string) (bool, error)
	MakeDirIfNotExist(dirName string) error
	IsFileExist(filename string) bool
	GetFileSize(fileName string) (int64, error)
	IsFileReadable(fileName string) (bool, error)
	IsFileWriteable(fileName string) (bool, error)
	PrintErrVerbose(any ...interface{})
	DoSync(ctx context.Context) error
	GetTotal() int64
}

type DSOptions func(*DirSync)

func WithVerbose(isVerbose bool) DSOptions {
	return func(ds *DirSync) {
		ds.IsVerbose = isVerbose
	}
}

func WithCreateEmptyFolder(createEmptyFolder bool) DSOptions {
	return func(ds *DirSync) {
		ds.CreateEmptyFolder = createEmptyFolder
	}
}

// New will create a directory sync object given the source and destination directories
func New(ctx context.Context, srcRoot string, dstRoot string, opts ...DSOptions) (DirSyncImpl, error) {
	absSrc, err := filepath.Abs(srcRoot)
	if err != nil {
		return nil, err
	}

	absDst, err := filepath.Abs(dstRoot)
	if err != nil {
		return nil, err
	}

	if absSrc == absDst {
		return nil, dsyncerr.ErrSameSourceDestination
	}

	ds := &DirSync{
		ctx:               ctx,
		SrcRoot:           srcRoot,
		DstRoot:           dstRoot,
		AbsSrcRoot:        absSrc,
		AbsDstRoot:        absDst,
		IsVerbose:         false,
		TotalFiles:        0,
		CreateEmptyFolder: false,
	}

	for _, opt := range opts {
		opt(ds)
	}

	return ds, nil
}

func (ds *DirSync) PrintErrVerbose(any ...interface{}) {
	if ds.IsVerbose {
		log.Printf("%+v\n", any)
	}
}

// IsEmptyDir will check if given dirName is empty directory
func (ds *DirSync) IsEmptyDir(dirName string) (bool, error) {
	file, err := os.Open(dirName)
	if err != nil {
		return false, err
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			ds.PrintErrVerbose(err)
		}
	}(file)

	_, err = file.Readdirnames(1) // Or f.Readdir(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}

// MakeDirIfNotExist will create a directory given by dirname if not exist
func (ds *DirSync) MakeDirIfNotExist(dirName string) error {
	// check if destination folder exist
	_, err := os.Stat(dirName)
	if os.IsNotExist(err) {
		// if not exist then create it
		err = os.Mkdir(dirName, 0755)
		if err != nil && os.IsNotExist(err) {
			ds.PrintErrVerbose("dirname:", dirName, "Err:", err)
			return err
		}
		ds.PrintErrVerbose(dirName, "successfully created")
	}
	return nil
}

// IsFileExist will check if file exist and return false if not
func (ds *DirSync) IsFileExist(filename string) bool {
	// check if destination folder exist
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

// WalkFiles will recursively list all the files and directories of a source root and checks
// if those files exist in destination root, if not then return the destination and the error
func (ds *DirSync) walkFiles(ctx context.Context, done <-chan struct{}, count chan<- int64) (<-chan InputData, <-chan error) {
	pathData := make(chan InputData)
	errC := make(chan error, 1)

	go func() {
		defer close(pathData)
		// WalkDir will recursively run through the directory for files and dirs
		errC <- filepath.WalkDir(ds.AbsSrcRoot, func(path string, d fs.DirEntry, err error) error {
			if path == ds.AbsSrcRoot {
				return nil // no need to check the root
			}
			// check the error
			if err != nil {
				// if not about permission error then return it for handling
				if !errors.Is(err, fs.ErrPermission) {
					return err
				}
				// if permission error then skip the file for further processing
				ds.PrintErrVerbose("Permission Err:", err, path, "will be skipped")
				return nil
			}

			// get the file info
			f, _err := d.Info()
			if _err != nil {
				ds.PrintErrVerbose("Fail getting file info Err:", err, path, "will be skipped")
				return _err // internal error
			}
			// prepare the destination path
			dstPath := fmt.Sprintf("%s%s", ds.AbsDstRoot, strings.TrimPrefix(path, ds.AbsSrcRoot))

			// if it is directory
			if f.IsDir() {
				// and check if empty
				isEmpty, errEmpty := ds.IsEmptyDir(path)
				if errEmpty != nil { // if we found error during checking, blacklist
					if !errors.Is(errEmpty, fs.ErrPermission) {
						return err
					}
					ds.PrintErrVerbose("Err:", errEmpty, path, "will be skipped")
					return nil
				}

				if isEmpty && !ds.CreateEmptyFolder { // skip if empty directory
					ds.PrintErrVerbose(path, "is empty folder, will be skipped")
					return nil
				}

				err = ds.MakeDirIfNotExist(dstPath)
				if err != nil {
					ds.PrintErrVerbose("fail create directory err:", err)
					return nil
				}

				// always skip directory
				return nil
			}

			readable, err := ds.IsFileReadable(path)
			if err != nil {
				ds.PrintErrVerbose("Readable error:", err, path, "will be skipped")
				return err // internal error
			}

			if !readable {
				ds.PrintErrVerbose(path, "cannot be read, will be skipped")
				return nil
			}

			id := InputData{path, dstPath, f.Size(), d.IsDir()}
			select {
			case pathData <- id:
			case <-ctx.Done():
				return errors.New("sync canceled")
			case <-done:
				return errors.New("sync canceled")
			}
			return nil
		})
	}()

	return pathData, errC
}

func (ds *DirSync) GetFileSize(fileName string) (int64, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return 0, err
	}
	defer func(f *os.File) {
		err = f.Close()
		if err != nil {
			ds.PrintErrVerbose(err)
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
		ds.PrintErrVerbose(err)
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
		ds.PrintErrVerbose(err)
	}
	return true, nil
}

// Checker will do mostly validation if a file is feasible to be copied
func (ds *DirSync) fileValidator(ctx context.Context, done <-chan struct{}, paths <-chan InputData, c chan<- result) {
	for fInput := range paths {
		// fmt.Println(fInput.srcPath, "-", fInput.dstPath)
		var err error
		if !fInput.isDir && ds.IsFileExist(fInput.dstPath) {
			// check the srcSize
			dstSize, err := ds.GetFileSize(fInput.dstPath)
			if err == nil {
				if dstSize == fInput.srcSize {
					dataSrc, err := ioutil.ReadFile(fInput.srcPath)
					if err != nil {
						// skip the file
						ds.PrintErrVerbose("read src file before md5 err:", err)
						continue
					}
					dataDst, err := ioutil.ReadFile(fInput.dstPath)
					if err != nil {
						// skip the file
						ds.PrintErrVerbose("read dst file before md5 err:", err)
						continue
					}
					if md5.Sum(dataSrc) == md5.Sum(dataDst) { //nolint:gosec
						// skip the file as identical
						continue
					}
				}
			}

			ds.PrintErrVerbose("err get size:", err)
			continue // skip
		}
		select {
		// list of files need to be copied
		case c <- result{fInput.srcPath, fInput.dstPath, err}:
			ds.PrintErrVerbose("sent", result{fInput.srcPath, fInput.dstPath, err})
		case <-ctx.Done():
			return
		case <-done:
			return
		}
	}
}

func (ds *DirSync) GetTotal() int64 {
	ds.lock.Lock()
	defer ds.lock.Unlock()
	return ds.TotalFiles
}

// DoSync will synchronize source and destination folders
// if context cancel is called then all operation stop accordingly
func (ds *DirSync) DoSync(ctx context.Context) error {
	done := make(chan struct{})
	defer close(done) // if close, all downstream will abandon its work

	count := make(chan int64)
	defer close(count)

	// level 1, walk the source directory recursively
	pathdata, errc := ds.walkFiles(ctx, done, count)

	res := make(chan result)
	var wg sync.WaitGroup

	// number of check workers to validate if need to do copy or no
	const numCheckers = WorkerCount
	wg.Add(numCheckers)
	for i := 0; i < numCheckers; i++ {
		go func() {
			//level 2 validate if file is valid for copy to destination
			ds.fileValidator(ctx, done, pathdata, res) // HLc
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(res)
	}()

	// set a goroutine to listen to number of changes
	go func() {
		for {
			select {
			case cnt := <-count:
				ds.lock.Lock()
				ds.TotalFiles = cnt
				ds.lock.Unlock()
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}()

	cnt := int64(0)
	//level 3 copy action
	for r := range res {
		if r.err != nil {
			ds.PrintErrVerbose("receive r.err:", r.err)
			continue
		}

		input, err := ioutil.ReadFile(r.sourcePath)
		if err != nil {
			ds.PrintErrVerbose("Error Read input:", err)
			return err
		}

		err = ioutil.WriteFile(r.destPath, input, 0755) //nolint:gosec
		if err != nil {
			ds.PrintErrVerbose("Error creating", r.destPath, "Err:", err)
			return err
		}
		cnt++
		count <- cnt
	}

	// Check whether the Walk failed.
	if err := <-errc; err != nil {
		ds.PrintErrVerbose("walkFiles err:", err)
		return err
	}
	count <- cnt
	// Return err
	return nil
}
