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
	"skasync/pkg/k8s"
	"skasync/pkg/skaffold"
	"skasync/pkg/sync"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Context,
	Namespace,
	RootDir string
	Pods     []k8s.PodConfig
	Sync     sync.Config
	Skaffold skaffold.Config
	API      api.Config
}

type envConfig struct {
	Context,
	Namespace string
}

type flagsConfig struct {
	Context,
	Namespace,
	ConfigFilePath string
}

func LoadConfig() (*Config, error) {
	currentDirPath, _ := os.Getwd()

	cfg := defaultConfig(currentDirPath)

	err := readEnvs(&cfg)
	if err != nil {
		return nil, err
	}

	flagsCfg, err := readFlags(currentDirPath)
	if err != nil {
		return nil, err
	}

	err = readFile(&cfg, flagsCfg.ConfigFilePath)
	if err != nil {
		return nil, err
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

	if len(cfg.Pods) == 0 {
		return nil, errors.New("undefined pods")
	}

	if err := k8s.CheckPodsCfg(cfg.Pods); err != nil {
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
	}
}

func readFlags(rootDirPath string) (*flagsConfig, error) {
	cfg := &flagsConfig{}

	configPath := filepath.Join(rootDirPath, "skasync.config.json")

	flag.StringVar(&cfg.ConfigFilePath, "c", configPath, "Config file")
	flag.StringVar(&cfg.Context, "context", "", "Using kubctl context")
	flag.StringVar(&cfg.Namespace, "ns", "", "Using kubctl namespace")

	flag.Parse()

	return cfg, nil
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

	for i, pod := range cfg.Pods {
		if filepath.IsAbs(pod.DockerfileDir) {
			continue
		}

		pod.DockerfileDir = filepath.Join(configFileDir, pod.DockerfileDir)
		if _, err := os.Stat(pod.DockerfileDir); os.IsNotExist(err) {
			return fmt.Errorf("pod (image \"%s\") dockerfile path \"%s\" is undefined", pod.Artifact, pod.DockerfileDir)
		}

		cfg.Pods[i] = pod
	}

	return nil
}
