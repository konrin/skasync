package filesystem

import (
	"archive/tar"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type headerModifier func(*tar.Header)

type TarProcessInfo struct {
	AllFilesCount,
	SendedFilesCount int
	BytesSended int64
}

func CreateMappedTar(w io.Writer, root string, pathMap map[string]string, progressCh chan TarProcessInfo) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	allFilesCount := len(pathMap)

	i := 0
	for src, dst := range pathMap {
		bytesLen, err := addFileToTar(root, src, dst, tw, nil)
		if err != nil {
			return err
		}

		if progressCh != nil {
			i++
			progressCh <- TarProcessInfo{allFilesCount, i, bytesLen}
		}
	}

	return nil
}

func addFileToTar(root string, src string, dst string, tw *tar.Writer, hm headerModifier) (int64, error) {
	fi, err := os.Lstat(src)
	if err != nil {
		return 0, err
	}

	mode := fi.Mode()
	if mode&os.ModeSocket != 0 {
		return 0, nil
	}

	var header *tar.Header
	if mode&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return 0, err
		}

		if filepath.IsAbs(target) {
			log.Printf("Skipping %s. Only relative symlinks are supported.", src)
			return 0, nil
		}

		header, err = tar.FileInfoHeader(fi, target)
		if err != nil {
			return 0, err
		}
	} else {
		header, err = tar.FileInfoHeader(fi, "")
		if err != nil {
			return 0, err
		}
	}

	if dst == "" {
		tarPath, err := filepath.Rel(root, src)
		if err != nil {
			return 0, err
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
		return 0, err
	}

	if mode.IsRegular() {
		f, err := os.Open(src)
		if err != nil {
			return 0, err
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return 0, fmt.Errorf("writing real file %q: %w", src, err)
		}

		return fi.Size(), nil
	}

	return 0, nil
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

type TarProcessInfoAverage struct {
	outCh chan TarProcessInfo

	mu          sync.Mutex
	inMap       map[string]TarProcessInfo
	bytesSended int64
}

func NewTarProcessInfoAverage(outCh chan TarProcessInfo) *TarProcessInfoAverage {
	return &TarProcessInfoAverage{
		outCh: outCh,
		inMap: make(map[string]TarProcessInfo),
	}
}

func (as *TarProcessInfoAverage) Set(id string, value TarProcessInfo) {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.inMap[id] = value

	avg := TarProcessInfo{}

	for _, item := range as.inMap {
		avg.AllFilesCount += item.AllFilesCount
		avg.SendedFilesCount += item.SendedFilesCount
		as.bytesSended += item.BytesSended
	}

	avg.BytesSended = as.bytesSended

	as.outCh <- avg
}
