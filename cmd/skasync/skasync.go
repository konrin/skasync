package main

import (
	"log"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	if cfg.Mode == WatcherMode {
		RunWatcher(cfg)
		return
	}

	RunSync(cfg)
}
