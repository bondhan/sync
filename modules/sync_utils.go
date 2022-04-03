package dirsync

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	dirsyncerr "github.com/bondhan/sync/pkg/errors"
	"github.com/bondhan/sync/pkg/model"
	"io"
	"os"
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

func QuickSort(a []string) []string {
	if len(a) < 2 {
		return a
	}
	pivot := len(a) - 1
	left := 0
	for i := range a {
		if a[i] < a[pivot] {
			a[left], a[i] = a[i], a[left]
			left++
		}
	}
	a[pivot], a[left] = a[left], a[pivot]
	QuickSort(a[:left])
	QuickSort(a[left+1:])
	return a
}

func CompareFile(src model.DirSync, dst map[string]model.DirSync) (bool, error) {

	return false, errors.New("xxx")

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

func ProcessDirSync(ctx context.Context, src map[string]model.DirSync,
	dest map[string]model.DirSync) (map[string]model.DirSync, error) {
	const workerCount = 4
	var err error

	ctx, cancelFunc := context.WithCancel(ctx)
	_ = cancelFunc

	diff := make(map[string]model.DirSync)
	done := make(chan struct{}, workerCount)
	sFile := make(chan model.DirSync)
	errC := make(chan error)

	var wg sync.WaitGroup
	wg.Add(1)

	go func(w *sync.WaitGroup) {
		defer w.Done()
		select {
		case _err, ok := <-errC:
			if !ok {
				fmt.Println("errC closed")
			} else {
				err = _err
				close(errC)
			}
			break
		}
	}(&wg)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go ComputeDirSync(i, ctx, &wg, done, sFile, errC, dest, diff)
	}

	for _, v := range src {
		sFile <- v
	}
	close(done)
	close(sFile)
	close(errC)

	fmt.Println("wg wait")
	wg.Wait()

	fmt.Println("all closed")

	return diff, err
}

func Print(list map[string]model.DirSync) {
	for _, v := range list {
		fmt.Println(v.Name, v.Size, "bytes", v.ModTime)
	}
}

func PrintSorted(list map[string]model.DirSync) {
	keys := make([]string, 0, len(list))
	for k := range list {
		keys = append(keys, k)
	}

	sortedKeys := QuickSort(keys)

	for _, k := range sortedKeys {
		v := list[k]
		fmt.Println(v.Name, v.Size, "bytes", v.ModTime)
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
