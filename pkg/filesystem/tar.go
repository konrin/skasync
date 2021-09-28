package filesystem

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

type headerModifier func(*tar.Header)

func CreateMappedTar(w io.Writer, root string, pathMap map[string]string, progressCh chan int) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	i := 0
	for src, dst := range pathMap {
		if err := addFileToTar(root, src, dst, tw, nil); err != nil {
			return err
		}

		if progressCh != nil {
			i++
			progressCh <- (i * 100) / len(pathMap)
		}
	}

	return nil
}

func addFileToTar(root string, src string, dst string, tw *tar.Writer, hm headerModifier) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}

	mode := fi.Mode()
	if mode&os.ModeSocket != 0 {
		return nil
	}

	var header *tar.Header
	if mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}

		if filepath.IsAbs(target) {
			log.Printf("Skipping %s. Only relative symlinks are supported.", src)
			return nil
		}

		header, err = tar.FileInfoHeader(fi, target)
		if err != nil {
			return err
		}
	} else {
		header, err = tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
	}

	if dst == "" {
		tarPath, err := filepath.Rel(root, src)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(tarPath)
	} else {
		header.Name = filepath.ToSlash(dst)
	}

	// Code copied from https://github.com/moby/moby/blob/master/pkg/archive/archive_windows.go
	if runtime.GOOS == "windows" {
		header.Mode = int64(chmodTarEntry(os.FileMode(header.Mode)))
	}
	if hm != nil {
		hm(header)
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if mode.IsRegular() {
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("writing real file %q: %w", src, err)
		}
	}

	return nil
}

// Code copied from https://github.com/moby/moby/blob/master/pkg/archive/archive_windows.go
func chmodTarEntry(perm os.FileMode) os.FileMode {
	// perm &= 0755 // this 0-ed out tar flags (like link, regular file, directory marker etc.)
	permPart := perm & os.ModePerm
	noPermPart := perm &^ os.ModePerm
	// Add the x bit: make everything +x from windows
	permPart |= 0111
	permPart &= 0755

	return noPermPart | permPart
}
