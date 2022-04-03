package dsync

import (
	"context"
	"errors"
	"fmt"
	dsyncerr "github.com/bondhan/sync/modules/errors"
	"log"
	"math/rand"
	"os"
	"testing"
	"time"
)

const (
	sourceDir      = "/tmp/source"
	destinationDir = "/tmp/destination"
)

var (
	randomFileName string
)

func writeFile(target string, content string) {
	err := os.WriteFile(target, //nolint:gosec
		[]byte(content), 0755)
	if err != nil {
		log.Fatal(err)
	}
}

func randomString(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

//https://stackoverflow.com/questions/37932551/mkdir-if-not-exists-using-golang
func ensureDir(dirName string) error {
	err := os.Mkdir(dirName, 0755)
	if err == nil {
		return nil
	}
	if os.IsExist(err) {
		// check that the existing path is a directory
		info, err := os.Stat(dirName)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("path exists but is not a directory")
		}
		return nil
	}
	return err
}

func TestMain(m *testing.M) {
	if err := os.RemoveAll(sourceDir); err != nil {
		log.Fatal(err)
	}
	if err := os.RemoveAll(destinationDir); err != nil {
		log.Fatal(err)
	}
	if err := ensureDir(sourceDir); err != nil {
		log.Fatal(err)
	}
	if err := ensureDir(destinationDir); err != nil {
		log.Fatal(err)
	}

	randomFileName = randomString(10)

	defer func() {
		if err := os.RemoveAll(sourceDir); err != nil {
			log.Fatal(err)
		}
		if err := os.RemoveAll(destinationDir); err != nil {
			log.Fatal(err)
		}
	}()

	os.Exit(m.Run())
}

func TestNew(t *testing.T) {
	verboseOpt := WithVerbose(false)
	emptyFolderOpt := WithCreateEmptyFolder(false)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		got, err := New(ctx, sourceDir, destinationDir, verboseOpt, emptyFolderOpt)

		if err != nil {
			t.Errorf("err should not be nil")
		}
		if got == nil {
			t.Errorf("got should not be nil")
		}
	})

	t.Run("fail due to source value is the same with destination", func(t *testing.T) {
		got, err := New(ctx, sourceDir, sourceDir, verboseOpt, emptyFolderOpt)

		if !errors.Is(err, dsyncerr.ErrSameSourceDestination) {
			t.Errorf("err should not be %s", dsyncerr.ErrSameSourceDestination)
		}
		if got != nil {
			t.Errorf("got should be nil")
		}
	})
}

func TestIsEmptyDir(t *testing.T) {
	verboseOpt := WithVerbose(false)
	emptyFolderOpt := WithCreateEmptyFolder(false)

	t.Run("dir is empty", func(t *testing.T) {
		ctx := context.Background()

		ds, err := New(ctx, sourceDir, destinationDir, verboseOpt, emptyFolderOpt)
		if err != nil {
			t.Errorf("fail test")
		}
		isEmpty, err := ds.IsEmptyDir(sourceDir)

		if err != nil {
			t.Errorf("err should be nil err: %s", err)
		}
		if !isEmpty {
			t.Errorf("should be empty")
		}
	})

	t.Run("fail no exist directory", func(t *testing.T) {
		ctx := context.Background()

		ds, err := New(ctx, sourceDir, destinationDir, verboseOpt, emptyFolderOpt)
		if err != nil {
			t.Errorf("fail test")
		}
		isEmpty, err := ds.IsEmptyDir(randomFileName)

		if err == nil {
			t.Errorf("err should not be nil")
		}
		if isEmpty {
			t.Errorf("should be empty")
		}
	})

	t.Run("dir not empty", func(t *testing.T) {
		target := fmt.Sprintf("%s/%s", sourceDir, randomFileName)
		writeFile(target, randomString(100))
		defer func(t string) {
			e := os.Remove(t)
			if e != nil {
				log.Fatal(e)
			}
		}(target)

		ctx := context.Background()

		ds, err := New(ctx, sourceDir, destinationDir, verboseOpt, emptyFolderOpt)
		if err != nil {
			t.Errorf("fail test")
		}
		isEmpty, err := ds.IsEmptyDir(sourceDir)

		if err != nil {
			t.Errorf("err should not be nil")
		}
		if isEmpty {
			t.Errorf("should no be empty")
		}
	})
}

func TestMakeDirIfNotExist(t *testing.T) {
	verboseOpt := WithVerbose(false)
	emptyFolderOpt := WithCreateEmptyFolder(false)

	t.Run("success", func(t *testing.T) {
		target := fmt.Sprintf("%s/%s", sourceDir, randomFileName)

		ctx := context.Background()

		ds, err := New(ctx, sourceDir, destinationDir, verboseOpt, emptyFolderOpt)
		if err != nil {
			t.Errorf("fail test")
		}
		err = ds.MakeDirIfNotExist(target)
		if err != nil {
			t.Errorf("should be success")
		}
	})
}

func TestIsFileExist(t *testing.T) {
	verboseOpt := WithVerbose(false)
	emptyFolderOpt := WithCreateEmptyFolder(false)

	t.Run("success", func(t *testing.T) {
		target := fmt.Sprintf("%s/%s", sourceDir, randomString(5))

		ctx := context.Background()

		ds, err := New(ctx, sourceDir, destinationDir, verboseOpt, emptyFolderOpt)
		if err != nil {
			t.Errorf("fail test")
		}
		isExist := ds.IsFileExist(target)
		if isExist {
			t.Errorf("file must not be exist")
		}
	})
	t.Run("success", func(t *testing.T) {
		target := fmt.Sprintf("%s/%s", sourceDir, randomString(5))
		writeFile(target, randomString(100))

		defer func(t string) {
			os.RemoveAll(t)
		}(target)

		ctx := context.Background()

		ds, err := New(ctx, sourceDir, destinationDir, verboseOpt, emptyFolderOpt)
		if err != nil {
			t.Errorf("fail test")
		}
		isExist := ds.IsFileExist(target)
		if !isExist {
			t.Errorf("file must not be exist")
		}
	})

}
