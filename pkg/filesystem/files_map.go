package filesystem

import (
	"io/fs"
	"path/filepath"
)

type FilesMap map[string]fs.FileInfo

type FilesMapService struct {
	rootDir string
}

func NewFilesMapService(rootDir string) *FilesMapService {
	return &FilesMapService{
		rootDir: rootDir,
	}
}

func (fms *FilesMapService) Walk() (FilesMap, error) {
	return fms.WalkForSubpath(fms.rootDir)
}

func (fms *FilesMapService) WalkForSubpath(path string) (FilesMap, error) {
	fsMap := make(FilesMap)

	filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if err != nil {
			// to log
			return nil
		}

		fsMap[path] = info

		return nil
	})

	return fsMap, nil
}

func (fm FilesMap) Copy() FilesMap {
	newFsMap := make(FilesMap)

	for key := range fm {
		newFsMap[key] = fm[key]
	}

	return newFsMap
}

func (fm FilesMap) ToSlice() []string {
	paths := make([]string, 0, len(fm))

	for path := range fm {
		paths = append(paths, path)
	}

	return paths
}
