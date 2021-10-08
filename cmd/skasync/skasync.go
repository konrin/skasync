package main

import (
	"fmt"
	"log"
	"os"
	"skasync/cmd/skasync/version"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	if cfg.Mode == VersionMode {
		fmt.Printf("Version: v%s\n", version.VERSION)
		os.Exit(0)
	}

	if cfg.Mode == WatcherMode {
		RunWatcher(cfg)
		return
	}

	RunSync(cfg)
}
