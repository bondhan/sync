package dirsync

import (
	"crypto/sha256"
	"fmt"
	dirsyncerr "github.com/bondhan/sync/pkg/errors"
	"github.com/bondhan/sync/pkg/model"
	"io"
	"log"
	"os"
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

func ComputeDiff(src map[string]model.DirSync, dest map[string]model.DirSync) (map[string]model.DirSync, error) {
	diff := make(map[string]model.DirSync)

	for k, v := range src {
		dst, ok := dest[k]
		if !ok {
			diff[k] = v
			continue
		}

		if v.Size != dst.Size {
			diff[k] = v
			continue
		}

		srcSha, err := CalcSHA256(v.Name)
		if err != nil {
			return nil, err
		}
		dstSha, err := CalcSHA256(dst.Name)
		if err != nil {
			return nil, err
		}
		if srcSha != dstSha {
			diff[k] = v
			continue
		}
	}

	return diff, nil
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
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return string(h.Sum(nil)), nil
}
