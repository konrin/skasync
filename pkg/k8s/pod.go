package k8s

import (
	"fmt"
	"skasync/pkg/cli"
	"skasync/pkg/docker"
	"sync"
)

type PodConfig struct {
	Artifact,
	Selector,
	Container,
	DockerfileDir,
	RootDir string
}

func CheckPodsCfg(pods []PodConfig) error {
	for _, podCfg := range pods {
		if len(podCfg.Artifact) == 0 {
			return fmt.Errorf("pod requery atrifact name: %+v", podCfg)
		}

		if len(podCfg.RootDir) == 0 {
			return fmt.Errorf("pod \"%s\" requery root dir", podCfg.Artifact)
		}

		if len(podCfg.Container) == 0 {
			return fmt.Errorf("pod \"%s\" requery container name", podCfg.Artifact)
		}
	}

	return nil
}

func DefaultPodConfig() PodConfig {
	return PodConfig{}
}

type Pod struct {
	Name,
	Container,
	Artifact,
	RootDir string
	DockerIgnorePredicate docker.Predicate
}

type PodsCtrl struct {
	rootDir string
	podsCfg []PodConfig
	kubeCtl *cli.KubeCtl

	mu   sync.Mutex
	pods map[string]*Pod
}

func NewPodsCtrl(rootDir string, podsCfg []PodConfig, kubeCtl *cli.KubeCtl) *PodsCtrl {
	return &PodsCtrl{
		rootDir: rootDir,
		podsCfg: podsCfg,
		kubeCtl: kubeCtl,
		pods:    make(map[string]*Pod),
	}
}

func (pc *PodsCtrl) register(podCfg PodConfig) error {
	if len(podCfg.Artifact) == 0 {
		return fmt.Errorf("pod requery atrifact name: %+v", podCfg)
	}

	if len(podCfg.RootDir) == 0 {
		return fmt.Errorf("pod \"%s\" requery root dir", podCfg.Artifact)
	}

	if len(podCfg.Container) == 0 {
		return fmt.Errorf("pod \"%s\" requery container name", podCfg.Artifact)
	}

	if _, ok := pc.pods[podCfg.Artifact]; ok {
		return fmt.Errorf("pod \"%s\" already exist", podCfg.Artifact)
	}

	podName, err := pc.kubeCtl.GetPodName(podCfg.Selector)
	if err != nil {
		return err
	}

	dockerfileDir := podCfg.DockerfileDir
	if len(dockerfileDir) == 0 {
		dockerfileDir = pc.rootDir
	}

	ignoreList, err := docker.GetIgnoreList(pc.rootDir, dockerfileDir)
	if err != nil {
		return err
	}

	dockerIgnorePredicate, err := docker.NewDockerIgnorePredicate(pc.rootDir, ignoreList)
	if err != nil {
		return err
	}

	newPod := &Pod{
		Name:                  podName,
		Container:             podCfg.Container,
		Artifact:              podCfg.Artifact,
		DockerIgnorePredicate: dockerIgnorePredicate,
		RootDir:               podCfg.RootDir,
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.pods[podCfg.Artifact] = newPod

	return nil
}

func (pc *PodsCtrl) Refresh() error {
	pc.mu.Lock()
	pc.pods = make(map[string]*Pod)
	pc.mu.Unlock()

	wg := sync.WaitGroup{}

	var errs []error = make([]error, 0)

	for _, podCfg := range pc.podsCfg {
		wg.Add(1)
		go func(podCfg PodConfig) {
			if err := pc.register(podCfg); err != nil {
				errs = append(errs, err)
			}
			wg.Done()
		}(podCfg)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("%+v", errs)
	}

	return nil
}

func (pc *PodsCtrl) GetPods() []*Pod {
	pods := make([]*Pod, 0, len(pc.pods))

	for key := range pc.pods {
		pods = append(pods, pc.pods[key])
	}

	return pods
}
