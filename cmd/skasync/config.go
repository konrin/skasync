package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"skasync/cmd/skasync/api"
	"skasync/pkg/docker"
	"skasync/pkg/k8s"
	"skasync/pkg/skaffold"
	"skasync/pkg/sync"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

const (
	WatcherMode = "watcher"
	SyncMode    = "sync"
	VersionMode = "version"
)

const (
	InSyncDiraction uint = iota
	OutSyncDiraction
)

type Config struct {
	Context,
	Namespace,
	RootDir string
	Mode      string
	IsDebug   bool
	Artifacts map[string]docker.ArtifactConfig
	Endpoints map[string]k8s.EndpointConfig
	Sync      sync.Config
	Skaffold  skaffold.Config
	API       api.Config
	SyncArgs  SyncArgs
}

type SyncArgs struct {
	SyncDiraction uint
	SyncInArgs    SyncInArgs
	SyncOutArgs   SyncOutArgs
}

type SyncInArgs struct {
	Pods      []string
	IsAllPods bool
	Paths     []string
}

type SyncOutArgs struct{}

type envConfig struct {
	Context,
	Namespace string
}

type flagsConfig struct {
	Context,
	Namespace,
	ConfigFilePath string
	IsDebug bool
}

func LoadConfig() (*Config, error) {
	currentDirPath, _ := os.Getwd()

	cfg := defaultConfig(currentDirPath)

	err := readEnvs(&cfg)
	if err != nil {
		return nil, err
	}

	err = readMode(&cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Mode == VersionMode {
		return &cfg, nil
	}

	flagsCfg, err := readFlags(cfg.Mode, currentDirPath)
	if err != nil {
		return nil, err
	}

	cfg.IsDebug = flagsCfg.IsDebug

	err = readFile(&cfg, flagsCfg.ConfigFilePath)
	if err != nil {
		return nil, err
	}

	if cfg.Mode == SyncMode {
		err = readSyncArgs(&cfg)
		if err != nil {
			return nil, err
		}
	}

	if len(cfg.Context) == 0 {
		cfg.Context = flagsCfg.Context
	}

	if len(cfg.Namespace) == 0 {
		cfg.Namespace = flagsCfg.Namespace
	}

	if len(cfg.Context) == 0 {
		return nil, errors.New("undefined context")
	}

	if len(cfg.Namespace) == 0 {
		return nil, errors.New("undefined namespace")
	}

	if len(cfg.Endpoints) == 0 {
		return nil, errors.New("undefined endpoints")
	}

	if err := k8s.CheckEndpointsCfg(cfg.Endpoints); err != nil {
		return nil, err
	}

	if err := docker.CheckArtifactsCfg(cfg.Artifacts); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func defaultConfig(rootDirPath string) Config {
	return Config{
		Sync:     sync.DefaultConfig(),
		Skaffold: skaffold.DefaultConfig(),
		API:      api.DefaultConfig(),
		RootDir:  rootDirPath,
		IsDebug:  false,
	}
}

func readFlags(mode, rootDirPath string) (*flagsConfig, error) {
	cfg := &flagsConfig{}

	argBais := 2
	for i, arg := range os.Args {
		if arg[0] == '-' {
			argBais = i
			break
		}
	}

	configPath := filepath.Join(rootDirPath, "skasync.config.json")

	flagSet := flag.NewFlagSet("config", flag.ContinueOnError)

	flagSet.StringVar(&cfg.ConfigFilePath, "c", configPath, "Config file")
	flagSet.StringVar(&cfg.Context, "context", "", "Using kubctl context")
	flagSet.StringVar(&cfg.Namespace, "ns", "", "Using kubctl namespace")
	flagSet.BoolVar(&cfg.IsDebug, "debug", false, "Set debug mode")

	flagSet.Parse(os.Args[argBais:])

	if !filepath.IsAbs(cfg.ConfigFilePath) {
		cfg.ConfigFilePath = filepath.Join(rootDirPath, cfg.ConfigFilePath)
	}

	return cfg, nil
}

func readMode(cfg *Config) error {
	if len(os.Args) < 2 {
		return errors.New("mode not found")
	}

	mode := os.Args[1]

	switch mode {
	case WatcherMode:
		cfg.Mode = WatcherMode
	case SyncMode:
		cfg.Mode = SyncMode
	case VersionMode:
		cfg.Mode = VersionMode
	default:
		return errors.New("undefined mode: " + mode)
	}

	return nil
}

func readEnvs(cfg *Config) error {
	envCfg := envConfig{}

	err := envconfig.Process("skasync", &envCfg)
	if err != nil {
		return err
	}

	if len(envCfg.Context) > 0 {
		cfg.Context = envCfg.Context
	}

	if len(envCfg.Namespace) > 0 {
		cfg.Namespace = envCfg.Namespace
	}

	return nil
}

func readSyncArgs(cfg *Config) error {
	if len(os.Args) < 4 {
		return errors.New("args length error")
	}

	switch os.Args[2] {
	case "in":
		pods := os.Args[3]
		if pods == "all" {
			cfg.SyncArgs.SyncInArgs.IsAllPods = true
		} else {
			cfg.SyncArgs.SyncInArgs.Pods = strings.Split(pods, ",")
		}

		cfg.SyncArgs.SyncInArgs.Paths = strings.Split(os.Args[4], ",")
	case "out":
	default:
		return fmt.Errorf("sync diraction %s is undefined", os.Args[2])
	}

	return nil
}

func readFile(cfg *Config, configFilePath string) error {
	currentPath, err := os.Getwd()
	if err != nil {
		return err
	}

	if !filepath.IsAbs(configFilePath) {
		configFilePath = filepath.Join(currentPath, configFilePath)
	}

	fileData, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(fileData, cfg)
	if err != nil {
		return err
	}

	configFileDir := filepath.Dir(configFilePath)

	if !filepath.IsAbs(cfg.RootDir) {
		cfg.RootDir = filepath.Join(configFileDir, cfg.RootDir)
	}

	if _, err := os.Stat(cfg.RootDir); os.IsNotExist(err) {
		return fmt.Errorf("root dir \"%s\" is undefined", cfg.RootDir)
	}

	for i, artifact := range cfg.Artifacts {
		if filepath.IsAbs(artifact.DockerfileDir) {
			continue
		}

		artifact.DockerfileDir = filepath.Join(cfg.RootDir, artifact.DockerfileDir)
		if _, err := os.Stat(artifact.DockerfileDir); os.IsNotExist(err) {
			return fmt.Errorf("artifact (image \"%s\") dockerfile path \"%s\" is undefined", artifact.Image, artifact.DockerfileDir)
		}

		cfg.Artifacts[i] = artifact
	}

	return nil
}
