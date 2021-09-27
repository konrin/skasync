package filesystem

type RefFilesMap struct {
	list            FilesMap
	filesMapService *FilesMapService
}

func NewRefFilesMap(filesMapService *FilesMapService) *RefFilesMap {
	return &RefFilesMap{
		list:            make(FilesMap),
		filesMapService: filesMapService,
	}
}

func (rfm *RefFilesMap) Refresh() error {
	fsMap, err := rfm.filesMapService.Walk()
	if err != nil {
		return err
	}

	rfm.list = fsMap

	return nil
}

func (rfm *RefFilesMap) HasFile(filePath string) bool {
	_, ok := rfm.list[filePath]
	return ok
}
