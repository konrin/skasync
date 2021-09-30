package main

import (
	"context"
	"log"
	"skasync/pkg/cli"
	"skasync/pkg/filesystem"
	"skasync/pkg/k8s"
	"skasync/pkg/sync"
	"strings"

	"github.com/schollz/progressbar/v3"
)

// skasync sync -> * to/path
// skasync sync <- podName to/path
func RunSync(cfg *Config) {
	mainCtx := context.Background()

	ccli := cli.NewCLI(cfg.Context, cfg.Namespace)
	kubeCtl := cli.NewKubeCtl(ccli)
	podsCtrl := k8s.NewPodsCtrl(cfg.RootDir, cfg.Pods, kubeCtl)
	refFilesMapService := filesystem.NewFilesMapService(cfg.RootDir)
	podSyncker := sync.NewPodSyncer(cfg.RootDir, ccli, podsCtrl, refFilesMapService)

	if err := podsCtrl.Refresh(); err != nil {
		log.Fatal(err)
	}

	if cfg.SyncArgs.SyncDiraction == InSyncDiraction {
		inSyncDiraction(mainCtx, cfg.SyncArgs, podsCtrl, podSyncker)
		return
	}

	outSyncDiraction(mainCtx)
}

func inSyncDiraction(ctx context.Context, cfg SyncArgs, podsCtrl *k8s.PodsCtrl, podSyncker *sync.PodSyncer) {
	var pods []*k8s.Pod

	if cfg.SyncInArgs.IsAllPods {
		pods = podsCtrl.GetPods()
	} else {
		pods = make([]*k8s.Pod, 0, len(cfg.SyncInArgs.Pods))

		for _, podArg := range cfg.SyncInArgs.Pods {
			podSp := strings.Split(podArg, ":")
			if len(podSp) != 2 {
				log.Fatalf("not found container name in pod %s", podArg)
			}

			pod, err := podsCtrl.Find(podSp[0], podSp[1])
			if err != nil {
				log.Fatal(err)
			}

			pods = append(pods, pod)
		}
	}

	progressCh := make(chan filesystem.TarProcessInfo, 10)
	bar := progressbar.Default(1)
	go func() {
		for {
			tarProcessInfo := <-progressCh
			bar.ChangeMax(tarProcessInfo.AllFilesCount)
			bar.Set(tarProcessInfo.SendedFilesCount)
			// bar.Describe(util.LenReadable(tarProcessInfo.BytesSended, 2))
		}
	}()

	podSyncker.SyncLocalPathsToPods(pods, cfg.SyncInArgs.Paths, progressCh)

	bar.Finish()
}

func outSyncDiraction(ctx context.Context) {
	panic("not implement")
}
