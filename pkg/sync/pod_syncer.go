package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	rootDir     string
	cli         *cli.CLI
	refFilesMap *filesystem.RefFilesMap
	podsCtrl    *k8s.PodsCtrl
}

func NewPodSyncer(rootDir string, cli *cli.CLI, podsCtrl *k8s.PodsCtrl, refFilesMap *filesystem.RefFilesMap) *PodSyncer {
	return &PodSyncer{
		rootDir:     rootDir,
		cli:         cli,
		podsCtrl:    podsCtrl,
		refFilesMap: refFilesMap,
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

func (k *PodSyncer) do(changeFiles []string) {
	countChangedFiles := util.SafeCounter{}

	changeList := filemon.ChangeFilesToChangeListConverter(changeFiles)

	wg := sync.WaitGroup{}

	for _, pod := range k.podsCtrl.GetPods() {
		wg.Add(1)

		go func(ppod *k8s.Pod) {
			changeFilesCount := k.syncPod(ppod, changeList)
			countChangedFiles.Add(changeFilesCount)
			wg.Done()
		}(pod)
	}

	wg.Wait()

	if countChangedFiles.Value() > 0 {
		println("Watching for changes...")
	}
}

func (k *PodSyncer) syncPod(pod *k8s.Pod, changeList filemon.ChangeList) int {
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
			k.copyFile(context.Background(), pod, allowedModifiedFiles)
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

func (k *PodSyncer) copyFile(ctx context.Context, pod *k8s.Pod, filePaths []string) {
	syncFilesMap := localFilePathToSyncMapConverter(k.rootDir, pod.RootDir, filePaths)

	reader, writer := io.Pipe()
	go func() {
		if err := filesystem.CreateMappedTar(writer, "/", syncFilesMap); err != nil {
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