package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"skasync/cmd/skasync/api"
	"skasync/pkg/cli"
	"skasync/pkg/filemon"
	"skasync/pkg/filesystem"
	"skasync/pkg/k8s"
	"skasync/pkg/skaffold"
	"skasync/pkg/sync"
	"syscall"

	"github.com/labstack/echo/v4"
)

func RunWatcher(cfg *Config) {
	mainCtx := context.Background()

	watcherCh := make(chan []string, 100)
	skaffoldLayerCh := make(chan []string, 100)
	errorsCh := make(chan error, 1)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	ccli := cli.NewCLI(cfg.Context, cfg.Namespace)
	kubeCtl := cli.NewKubeCtl(ccli)
	podsCtrl := k8s.NewPodsCtrl(cfg.RootDir, cfg.Pods, kubeCtl)
	refFilesMapService := filesystem.NewFilesMapService(cfg.RootDir)
	// refFilesMap := filesystem.NewRefFilesMap(refFilesMapService)
	watcher := filemon.NewWatcher(cfg.RootDir, cfg.Sync.Debounce)
	podSyncker := sync.NewPodSyncer(cfg.RootDir, ccli, podsCtrl, refFilesMapService)
	skaffoldStatusProbe := skaffold.NewStatusProbe(cfg.Skaffold.Addr, podsCtrl)
	skaffoldStatusLayer := sync.NewSkaffoldStatusLayer(cfg.Skaffold.WatchingDeployStatus, skaffoldLayerCh, podsCtrl)

	go func() {
		errorsCh <- api.NewAPIListenerAndStart(cfg.API, func(e *echo.Echo) error {
			api.NewSyncController(e.Group("/sync"), podSyncker, podsCtrl)

			return nil
		})
	}()

	if err := podsCtrl.Refresh(); err != nil {
		log.Fatal(err)
	}

	skaffoldStatusProbe.Subscribe(skaffoldStatusLayer.StatusHandler)

	go func() {
		errorsCh <- watcher.Watch(mainCtx, watcherCh)
	}()

	go func() {
		errorsCh <- skaffoldStatusLayer.Do(mainCtx, watcherCh)
	}()

	go func() {
		errorsCh <- podSyncker.Do(mainCtx, skaffoldLayerCh)
	}()

	go func() {
		errorsCh <- skaffoldStatusProbe.Listen(mainCtx)
	}()

	println("Skasync is started")

	// Wait critical error or term signal
	select {
	case err := <-errorsCh:
		mainCtx.Done()
		log.Fatal(err)
	case <-sigChan:
		mainCtx.Done()
		println("Receive stop signal")
	}
}
