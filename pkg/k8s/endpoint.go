package k8s

import (
	"errors"
	"fmt"
	"skasync/pkg/cli"
	"skasync/pkg/docker"
	"sync"
)

type EndpointConfig struct {
	Artifact,
	Selector,
	Container,
	DockerfileDir,
	RootDir string
}

func CheckEndpointsCfg(pods map[string]EndpointConfig) error {
	for _, podCfg := range pods {
		if len(podCfg.Artifact) == 0 {
			return fmt.Errorf("pod require artifact id: %+v", podCfg)
		}

		if len(podCfg.Selector) == 0 {
			return errors.New("pod selector not found")
		}

		if len(podCfg.Container) == 0 {
			return fmt.Errorf("pod \"%s\" require container name", podCfg.Artifact)
		}
	}

	return nil
}

type Endpoint struct {
	TagName,
	PodName,
	Container string
	Artifact docker.Artifact
}

type EndpointCtrl struct {
	rootDir         string
	epsCfg          map[string]EndpointConfig
	kubeCtl         *cli.KubeCtl
	artifactService *docker.ArtifactService

	mu        sync.Mutex
	endpoints map[string]*Endpoint
}

func NewEndpointsCtrl(rootDir string, podsCfg map[string]EndpointConfig, kubeCtl *cli.KubeCtl, artifactService *docker.ArtifactService) *EndpointCtrl {
	return &EndpointCtrl{
		rootDir:         rootDir,
		epsCfg:          podsCfg,
		kubeCtl:         kubeCtl,
		artifactService: artifactService,
		endpoints:       make(map[string]*Endpoint),
	}
}

func (pc *EndpointCtrl) register(tagName string, epCfg EndpointConfig) error {
	podName, err := pc.kubeCtl.GetPodName(epCfg.Selector)
	if err != nil {
		return err
	}

	artifact, err := pc.artifactService.FindById(epCfg.Artifact)
	if err != nil {
		return err
	}

	if pc.HasEndpointExist(podName) {
		return fmt.Errorf("endpoint \"%s\" already exist", podName)
	}

	newEndpoint := &Endpoint{
		TagName:   tagName,
		PodName:   podName,
		Container: epCfg.Container,
		Artifact:  artifact,
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.endpoints[tagName] = newEndpoint

	return nil
}

func (pc *EndpointCtrl) HasEndpointExist(name string) bool {
	for _, p := range pc.endpoints {
		if p.PodName == name {
			return true
		}
	}
	return false
}

func (pc *EndpointCtrl) Refresh() error {
	pc.mu.Lock()
	pc.endpoints = make(map[string]*Endpoint)
	pc.mu.Unlock()

	wg := sync.WaitGroup{}

	var errs []error = make([]error, 0)

	for tagName, epCfg := range pc.epsCfg {
		wg.Add(1)
		go func(tagName string, epCfg EndpointConfig) {
			if err := pc.register(tagName, epCfg); err != nil {
				errs = append(errs, err)
			}
			wg.Done()
		}(tagName, epCfg)
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("%+v", errs)
	}

	return nil
}

func (pc *EndpointCtrl) GetPods() []*Endpoint {
	pods := make([]*Endpoint, 0, len(pc.endpoints))

	for key := range pc.endpoints {
		pods = append(pods, pc.endpoints[key])
	}

	return pods
}

// func (pc *EndpointCtrl) FindByArtifact(artifactName, container string) (*Endpoint, error) {
// 	for _, pod := range pc.endpoints {
// 		if pod.Artifact == artifactName && pod.Container == container {
// 			return pod, nil
// 		}
// 	}

// 	return nil, errors.New("endpoint not found")
// }

func (pc *EndpointCtrl) FindByTag(tagName string) (*Endpoint, error) {
	for _, pod := range pc.endpoints {
		if pod.TagName == tagName {
			return pod, nil
		}
	}

	return nil, errors.New("endpoint not found")
}
