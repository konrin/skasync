package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"skasync/pkg/cli"
	"skasync/pkg/docker"
	"skasync/pkg/filemon"
	"skasync/pkg/filesystem"
	"skasync/pkg/k8s"
	"skasync/pkg/util"
	"sync"
)

type EndpointSyncker struct {
	rootDir         string
	cli             *cli.CLI
	filesMapService *filesystem.FilesMapService
	podsCtrl        *k8s.EndpointCtrl
}

func NewEndpointSyncker(rootDir string, cli *cli.CLI, podsCtrl *k8s.EndpointCtrl, filesMapService *filesystem.FilesMapService) *EndpointSyncker {
	return &EndpointSyncker{
		rootDir:         rootDir,
		cli:             cli,
		podsCtrl:        podsCtrl,
		filesMapService: filesMapService,
	}
}

func (k *EndpointSyncker) Do(ctx context.Context, changeFilesCh chan filemon.ChangeList) error {
	for {
		select {
		case changeFiles := <-changeFilesCh:
			k.do(changeFiles)
		case <-ctx.Done():
			return nil
		}
	}
}

func (k *EndpointSyncker) SyncLocalPathToPod(pod *k8s.Endpoint, localPath string) error {
	absPath := filepath.Join(k.rootDir, localPath)

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return err
	}

	if !info.IsDir() {
		changeList := filemon.ChangeFilesToChangeListConverter([]string{absPath})

		k.syncEndpoint(pod, changeList, nil)
		return nil
	}

	filesMap, err := k.filesMapService.WalkForSubpath(absPath)
	if err != nil {
		return err
	}

	changeList := filemon.ChangeFilesToChangeListConverter(filesMap.ToSlice())

	k.syncEndpoint(pod, changeList, nil)

	return nil
}

func (k *EndpointSyncker) SyncLocalPathToPods(localPath string) error {
	wg := sync.WaitGroup{}
	for _, pod := range k.podsCtrl.GetPods() {
		wg.Add(1)
		go func(pod *k8s.Endpoint) {
			k.SyncLocalPathToPod(pod, localPath)
			wg.Done()
		}(pod)
	}

	wg.Wait()
	return nil
}

func (k *EndpointSyncker) SyncLocalPathsToPods(pods []*k8s.Endpoint, localPaths []string, progressCh chan filesystem.TarProcessInfo) error {
	filesMap := make(filesystem.FilesMap)

	for _, localPath := range localPaths {
		absPath := filepath.Join(k.rootDir, localPath)

		info, err := os.Stat(absPath)
		if os.IsNotExist(err) {
			return err
		}

		if !info.IsDir() {
			filesMap[absPath] = info
			continue
		}

		newFilesMap, err := k.filesMapService.WalkForSubpath(absPath)
		if err != nil {
			return err
		}

		filesMap.Append(newFilesMap)
	}

	changeList := filemon.ChangeFilesToChangeListConverter(filesMap.ToSlice())

	awgStream := filesystem.NewTarProcessInfoAverage(progressCh)

	wg := sync.WaitGroup{}
	for _, pod := range pods {
		wg.Add(1)
		go func(pod *k8s.Endpoint) {
			podProgressCh := make(chan filesystem.TarProcessInfo, 10)
			go func() {
				for {
					awgStream.Set(pod.TagName, <-podProgressCh)
				}
			}()
			k.syncEndpoint(pod, changeList, podProgressCh)
			wg.Done()
		}(pod)
	}

	wg.Wait()
	return nil
}

func (k *EndpointSyncker) do(changeList filemon.ChangeList) {
	countChangedFiles := util.SafeCounter{}

	// changeList := filemon.ChangeFilesToChangeListConverter(changeFiles)

	wg := sync.WaitGroup{}

	for _, pod := range k.podsCtrl.GetPods() {
		wg.Add(1)

		go func(_ep *k8s.Endpoint) {
			modifiedLen, deletedLen := k.syncEndpoint(_ep, changeList, nil)
			countChangedFiles.Add(modifiedLen + deletedLen)
			wg.Done()
		}(pod)
	}

	wg.Wait()

	if countChangedFiles.Value() > 0 {
		println("Watching for changes...")
	}
}

func (k *EndpointSyncker) syncEndpoint(pod *k8s.Endpoint, changeList filemon.ChangeList, progressCh chan filesystem.TarProcessInfo) (modifiedLen, deletedLen int) {
	allowedDeletedFiles := getAllowedDeletedFiles(changeList, pod.Artifact.DockerIgnorePredicate)
	allowedModifiedFiles := getAllowedModifiedFiles(changeList, pod.Artifact.DockerIgnorePredicate)

	allAllowedFiles := append(allowedModifiedFiles, allowedDeletedFiles...)

	allowedDeletedFiles, allowedModifiedFiles = filemon.CheckExistedFiles(allAllowedFiles...)

	changeFilesCount := len(allowedDeletedFiles) + len(allowedModifiedFiles)
	if changeFilesCount == 0 {
		return 0, 0
	}

	fmt.Printf(
		"\033[34mSyncing %d files\033[0m \033[37m[\033[0m\033[33m-%d ~%d\033[0m\033[37m]\033[0m \033[37mfor %s\033[0m\n",
		changeFilesCount,
		len(allowedDeletedFiles),
		len(allowedModifiedFiles),
		pod.TagName,
	)

	wg := sync.WaitGroup{}

	if len(allowedDeletedFiles) > 0 {
		wg.Add(1)
		go func() {
			k.deleteFile(context.Background(), pod, allowedDeletedFiles)
			wg.Done()
		}()
	}

	if len(allowedModifiedFiles) > 0 {
		wg.Add(1)
		go func() {
			k.copyFile(context.Background(), pod, allowedModifiedFiles, progressCh)
			wg.Done()
		}()
	}

	wg.Wait()

	return len(allowedModifiedFiles), len(allowedDeletedFiles)
}

func (k *EndpointSyncker) deleteFile(ctx context.Context, pod *k8s.Endpoint, filePaths []string) {
	args := make([]string, 0, 9+len(filePaths))
	args = append(args, pod.PodName, "-c", pod.Container, "--", "rm", "-rf", "--")
	for _, dst := range filePaths {
		args = append(args, userFilePathToPodFilePath(k.rootDir, pod.Artifact.RootDir, dst, true))
	}

	deleteCmd := k.cli.Command(context.Background(), "exec", args...)

	stderr := bytes.Buffer{}
	deleteCmd.Stderr = &stderr

	deleteCmd.Run()
	deleteCmd.Wait()

	if stderr.Len() > 0 {
		println(stderr.String())
	}
}

func (k *EndpointSyncker) copyFile(ctx context.Context, pod *k8s.Endpoint, filePaths []string, progressCh chan filesystem.TarProcessInfo) {
	syncFilesMap := localFilePathToSyncMapConverter(k.rootDir, pod.Artifact.RootDir, filePaths)

	reader, writer := io.Pipe()
	go func() {
		if err := filesystem.CreateMappedTar(writer, "/", syncFilesMap, progressCh); err != nil {
			writer.CloseWithError(err)
		} else {
			writer.Close()
		}
	}()

	copyCmd := k.cli.Command(
		context.Background(),
		"exec",
		pod.PodName,
		"-c",
		pod.Container, "-i", "--", "tar", "xmf", "-", "-C", "/", "--no-same-owner",
	)

	copyCmd.Stdin = reader

	stderr := bytes.Buffer{}
	copyCmd.Stderr = &stderr

	copyCmd.Run()
	copyCmd.Wait()

	// fmt.Printf("size: %s", util.LenReadable(0, 2))

	if stderr.Len() > 0 {
		println(stderr.String())
	}
}

func getAllowedModifiedFiles(changeList filemon.ChangeList, predicate docker.Predicate) []string {
	files := make([]string, 0)

	for filePath := range changeList.ModifiedAndAdded() {
		ok, err := predicate(filePath, nil)
		if ok || err != nil {
			continue
		}

		files = append(files, filePath)
	}

	return files
}

func getAllowedDeletedFiles(changeList filemon.ChangeList, predicate docker.Predicate) []string {
	files := make([]string, 0)

	for filePath := range changeList.Deleted() {
		ok, err := predicate(filePath, nil)
		if ok || err != nil {
			continue
		}

		files = append(files, filePath)
	}

	return files
}

func localFilePathToSyncMapConverter(rootDir, podRootPath string, files []string) map[string]string {
	list := make(map[string]string)

	for _, filePath := range files {
		list[filePath] = userFilePathToPodFilePath(rootDir, podRootPath, filePath, false)
	}

	return list
}

func userFilePathToPodFilePath(rootDir, podRootPath, userFilePath string, needFirstSlash bool) string {
	fileRel, err := filepath.Rel(rootDir, userFilePath)
	if err != nil {
		return userFilePath
	}

	podPath := filepath.Join(podRootPath, fileRel)
	if !needFirstSlash && podPath[0] == '/' {
		podPath = podPath[1:]
	}

	return podPath
}
