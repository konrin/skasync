package filesystem

import (
	"io/fs"
	"path/filepath"
	"skasync/pkg/docker"
)

type FilesMap map[string]fs.FileInfo

type FilesMapService struct {
	rootDir               string
	dockerIgnorePredicate docker.Predicate
}

func NewFilesMapService(rootDir string, dockerIgnorePredicate docker.Predicate) *FilesMapService {
	return &FilesMapService{
		rootDir:               rootDir,
		dockerIgnorePredicate: dockerIgnorePredicate,
	}
}

func (fms *FilesMapService) Walk() (FilesMap, error) {
	return fms.WalkForSubpath(fms.rootDir)
}

func (fms *FilesMapService) WalkForSubpath(path string) (FilesMap, error) {
	fsMap := make(FilesMap)

	filepath.Walk(fms.rootDir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if err != nil {
			// to log
			return nil
		}

		ok, err := fms.dockerIgnorePredicate(path, info)
		if err != nil {
			// to log
			return nil
		}

		if !ok {
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
