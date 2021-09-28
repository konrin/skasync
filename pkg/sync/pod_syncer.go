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

type PodSyncer struct {
	rootDir         string
	cli             *cli.CLI
	filesMapService *filesystem.FilesMapService
	podsCtrl        *k8s.PodsCtrl
}

func NewPodSyncer(rootDir string, cli *cli.CLI, podsCtrl *k8s.PodsCtrl, filesMapService *filesystem.FilesMapService) *PodSyncer {
	return &PodSyncer{
		rootDir:         rootDir,
		cli:             cli,
		podsCtrl:        podsCtrl,
		filesMapService: filesMapService,
	}
}

func (k *PodSyncer) Do(ctx context.Context, changeFilesCh chan []string) error {
	for {
		select {
		case changeFiles := <-changeFilesCh:
			k.do(changeFiles)
		case <-ctx.Done():
			return nil
		}
	}
}

func (k *PodSyncer) SyncLocalPathToPod(pod *k8s.Pod, localPath string) error {
	absPath := filepath.Join(k.rootDir, localPath)

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return err
	}

	if !info.IsDir() {
		changeList := filemon.ChangeFilesToChangeListConverter([]string{absPath})

		k.syncPod(pod, changeList, nil)
		return nil
	}

	filesMap, err := k.filesMapService.WalkForSubpath(absPath)
	if err != nil {
		return err
	}

	changeList := filemon.ChangeFilesToChangeListConverter(filesMap.ToSlice())

	k.syncPod(pod, changeList, nil)

	return nil
}

func (k *PodSyncer) SyncLocalPathToPods(localPath string) error {
	wg := sync.WaitGroup{}
	for _, pod := range k.podsCtrl.GetPods() {
		wg.Add(1)
		go func(pod *k8s.Pod) {
			k.SyncLocalPathToPod(pod, localPath)
			wg.Done()
		}(pod)
	}

	wg.Wait()
	return nil
}

func (k *PodSyncer) SyncLocalPathsToPods(pods []*k8s.Pod, localPaths []string, progressCh chan int) error {
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

	awgStream := util.NewAverageStream(progressCh)

	wg := sync.WaitGroup{}
	for _, pod := range pods {
		wg.Add(1)
		go func(pod *k8s.Pod) {
			podProgressCh := make(chan int, 10)
			go func() {
				for {
					awgStream.Set(pod.Artifact, <-podProgressCh)
				}
			}()
			k.syncPod(pod, changeList, podProgressCh)
			wg.Done()
		}(pod)
	}

	wg.Wait()
	return nil
}

func (k *PodSyncer) do(changeFiles []string) {
	countChangedFiles := util.SafeCounter{}

	changeList := filemon.ChangeFilesToChangeListConverter(changeFiles)

	wg := sync.WaitGroup{}

	for _, pod := range k.podsCtrl.GetPods() {
		wg.Add(1)

		go func(ppod *k8s.Pod) {
			changeFilesCount := k.syncPod(ppod, changeList, nil)
			countChangedFiles.Add(changeFilesCount)
			wg.Done()
		}(pod)
	}

	wg.Wait()

	if countChangedFiles.Value() > 0 {
		println("Watching for changes...")
	}
}

func (k *PodSyncer) syncPod(pod *k8s.Pod, changeList filemon.ChangeList, progressCh chan int) int {
	allowedDeletedFiles := getAllowedDeletedFiles(changeList, pod.DockerIgnorePredicate)
	allowedModifiedFiles := getAllowedModifiedFiles(changeList, pod.DockerIgnorePredicate)

	changeFilesCount := len(allowedDeletedFiles) + len(allowedModifiedFiles)
	if changeFilesCount == 0 {
		return 0
	}

	fmt.Printf("Syncing %d files for %s:%s -> %s\n", changeFilesCount, pod.Artifact, pod.Name, pod.Container)

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

	return changeFilesCount
}

func (k *PodSyncer) deleteFile(ctx context.Context, pod *k8s.Pod, filePaths []string) {
	args := make([]string, 0, 9+len(filePaths))
	args = append(args, pod.Name, "-c", pod.Container, "--", "rm", "-rf", "--")
	for _, dst := range filePaths {
		args = append(args, userFilePathToPodFilePath(k.rootDir, pod.RootDir, dst, true))
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

func (k *PodSyncer) copyFile(ctx context.Context, pod *k8s.Pod, filePaths []string, progressCh chan int) {
	syncFilesMap := localFilePathToSyncMapConverter(k.rootDir, pod.RootDir, filePaths)

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
		pod.Name,
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

func getAllowedModifiedFiles(changeList filemon.ChangeList, preicate docker.Predicate) []string {
	files := make([]string, 0)

	for filePath := range changeList.Modified {
		ok, err := preicate(filePath, nil)
		if ok || err != nil {
			continue
		}

		files = append(files, filePath)
	}

	return files
}

func getAllowedDeletedFiles(changeList filemon.ChangeList, preicate docker.Predicate) []string {
	files := make([]string, 0)

	for filePath := range changeList.Deleted {
		ok, err := preicate(filePath, nil)
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
