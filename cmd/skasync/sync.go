package main

import (
	"context"
	"log"
	"skasync/pkg/cli"
	"skasync/pkg/docker"
	"skasync/pkg/filesystem"
	"skasync/pkg/k8s"
	"skasync/pkg/sync"
	"skasync/pkg/util"

	"github.com/schollz/progressbar/v3"
)

// skasync sync -> * to/path
// skasync sync <- podName to/path
func RunSync(cfg *Config) {
	mainCtx := context.Background()

	ccli := cli.NewCLI(cfg.Context, cfg.Namespace)
	kubeCtl := cli.NewKubeCtl(ccli)
	artifactService := docker.NewArtifactService(cfg.RootDir)
	podsCtrl := k8s.NewEndpointsCtrl(cfg.RootDir, cfg.Endpoints, kubeCtl, artifactService)
	refFilesMapService := filesystem.NewFilesMapService(cfg.RootDir)
	podSyncker := sync.NewEndpointSyncker(cfg.RootDir, ccli, podsCtrl, refFilesMapService)

	if err := artifactService.Load(cfg.Artifacts); err != nil {
		log.Fatal(err)
	}

	if err := podsCtrl.Refresh(); err != nil {
		log.Fatal(err)
	}

	if cfg.SyncArgs.SyncDiraction == InSyncDiraction {
		inSyncDiraction(mainCtx, cfg.SyncArgs, podsCtrl, podSyncker)
		return
	}

	outSyncDiraction(mainCtx)
}

func inSyncDiraction(ctx context.Context, cfg SyncArgs, podsCtrl *k8s.EndpointCtrl, podSyncker *sync.EndpointSyncker) {
	var pods []*k8s.Endpoint

	if cfg.SyncInArgs.IsAllPods {
		pods = podsCtrl.GetPods()
	} else {
		pods = make([]*k8s.Endpoint, 0, len(cfg.SyncInArgs.Pods))

		for _, podArg := range cfg.SyncInArgs.Pods {
			pod, err := podsCtrl.FindByTag(podArg)
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
			bar.Describe(util.LenReadable(int(tarProcessInfo.BytesSended), 2))
		}
	}()

	podSyncker.SyncLocalPathsToPods(pods, cfg.SyncInArgs.Paths, progressCh)

	bar.Finish()
}

func outSyncDiraction(ctx context.Context) {
	panic("not implement")
}
