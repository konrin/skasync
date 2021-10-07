package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"skasync/cmd/skasync/api"
	"skasync/pkg/cli"
	"skasync/pkg/debug"
	"skasync/pkg/docker"
	"skasync/pkg/filemon"
	"skasync/pkg/filesystem"
	"skasync/pkg/git"
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
	filesChangeListCh := make(chan filemon.ChangeList, 10)
	errorsCh := make(chan error, 1)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	ccli := cli.NewCLI(cfg.Context, cfg.Namespace)
	kubeCtl := cli.NewKubeCtl(ccli)
	artifactService := docker.NewArtifactService(cfg.RootDir)
	endpointsCtrl := k8s.NewEndpointsCtrl(cfg.RootDir, cfg.Endpoints, kubeCtl, artifactService)
	refFilesMapService := filesystem.NewFilesMapService(cfg.RootDir)
	watcher := filemon.NewWatcher(cfg.RootDir, cfg.Sync.Debounce)
	endpointSyncker := sync.NewEndpointSyncker(cfg.RootDir, ccli, endpointsCtrl, refFilesMapService)
	skaffoldStatusProbe := skaffold.NewStatusProbe(cfg.Skaffold.Addr, endpointsCtrl)
	skaffoldStatusLayer := sync.NewSkaffoldStatusLayer(cfg.Skaffold.WatchingDeployStatus, skaffoldLayerCh, endpointsCtrl)
	gitCheckoutMon := git.NewCheckoutMon(cfg.RootDir)
	gateway := filemon.NewGateway(cfg.Sync.Debounce)
	debugChangeList := debug.NewChangeList()

	go func() {
		errorsCh <- gitCheckoutMon.Listen(mainCtx)
	}()

	go func() {
		fmt.Printf("API listening at: localhost:%d\n", cfg.API.Port)
		errorsCh <- api.NewAPIListenerAndStart(cfg.API, func(e *echo.Echo) error {
			api.NewSyncController(e.Group("/sync"), endpointSyncker, endpointsCtrl)
			api.NewDebugController(e.Group("/debug"), debugChangeList)
			return nil
		})
	}()

	if err := artifactService.Load(cfg.Artifacts); err != nil {
		log.Fatal(err)
	}

	if err := endpointsCtrl.Refresh(); err != nil {
		log.Fatal(err)
	}

	fsChangesCh := make(chan filemon.ChangeList, 10)
	gateway.RegisterProvider(mainCtx, "fs", fsChangesCh)

	gitCheckoutChangesCh := make(chan filemon.ChangeList, 10)
	gitCheckoutMon.Subscribe(func(cl filemon.ChangeList) {
		gitCheckoutChangesCh <- cl
	})
	gateway.RegisterProvider(mainCtx, "git.checkout", gitCheckoutChangesCh)

	gateway.Subscribe(func(m map[string]filemon.ChangeList) {
		a := filemon.GatewayResultToChangeList(m)

		if cfg.IsDebug {
			id := debugChangeList.AddResult(m)
			fmt.Printf("Change id: %d\n%s", id, filemon.ToStringGatewayResult(m))
		}

		filesChangeListCh <- a
	})

	skaffoldStatusProbe.Subscribe(skaffoldStatusLayer.StatusHandler)

	go func() {
		errorsCh <- watcher.Watch(mainCtx, watcherCh)
	}()

	go func() {
		errorsCh <- skaffoldStatusLayer.Do(mainCtx, watcherCh)
	}()

	go func() {
		errorsCh <- gateway.Start(mainCtx)
	}()

	go func() {
		for {
			fsChangesCh <- filemon.ChangeFilesToChangeListConverter(<-skaffoldLayerCh)
		}
	}()

	go func() {
		errorsCh <- endpointSyncker.Do(mainCtx, filesChangeListCh)
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
