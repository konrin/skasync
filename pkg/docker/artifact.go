package docker

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrArtifactNotFound  = errors.New("artifact not found")
	ErrArtifactIsExisted = errors.New("artifact is exist")
)

type ArtifactConfig struct {
	Image,
	RootDir,
	DockerfileDir string
}

type Artifact struct {
	Id,
	Image,
	RootDir string
	DockerIgnorePredicate Predicate
}

type ArtifactService struct {
	rootDir string

	mu   sync.Mutex
	list map[string]*Artifact
}

func NewArtifactService(rootDir string) *ArtifactService {
	return &ArtifactService{
		rootDir: rootDir,
		list:    make(map[string]*Artifact),
	}
}

func (as *ArtifactService) Load(artifacts map[string]ArtifactConfig) error {
	for id, artifact := range artifacts {
		if err := as.Register(id, artifact); err != nil {
			return err
		}
	}

	return nil
}

func (as *ArtifactService) Register(id string, cfg ArtifactConfig) error {
	if _, exist := as.list[id]; exist {
		return ErrArtifactIsExisted
	}

	ignoreList, err := GetIgnoreList(as.rootDir, cfg.DockerfileDir)
	if err != nil {
		return err
	}

	dockerIgnorePredicate, err := NewDockerIgnorePredicate(as.rootDir, ignoreList)
	if err != nil {
		return err
	}

	as.mu.Lock()
	as.list[id] = &Artifact{
		Id:                    id,
		Image:                 cfg.Image,
		RootDir:               cfg.RootDir,
		DockerIgnorePredicate: dockerIgnorePredicate,
	}
	as.mu.Unlock()

	return nil
}

func (as *ArtifactService) FindById(id string) (Artifact, error) {
	artifact, ok := as.list[id]
	if !ok {
		return Artifact{}, ErrArtifactNotFound
	}

	return *artifact, nil
}

func CheckArtifactsCfg(artifacts map[string]ArtifactConfig) error {
	for _, artifactCfg := range artifacts {
		if len(artifactCfg.Image) == 0 {
			return fmt.Errorf("pod require artifact name: %+v", artifactCfg)
		}
	}

	return nil
}
