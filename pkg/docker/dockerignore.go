package docker

import (
	"os"
	"path/filepath"

	"github.com/moby/buildkit/frontend/dockerfile/dockerignore"
)

func GetIgnoreList(workspace string, absDockerfilePath string) ([]string, error) {
	var excludes []string
	dockerignorePaths := []string{
		absDockerfilePath + ".dockerignore",
		filepath.Join(workspace, ".dockerignore"),
	}
	for _, dockerignorePath := range dockerignorePaths {
		if _, err := os.Stat(dockerignorePath); !os.IsNotExist(err) {
			r, err := os.Open(dockerignorePath)
			if err != nil {
				return nil, err
			}
			defer r.Close()

			excludes, err = dockerignore.ReadAll(r)
			if err != nil {
				return nil, err
			}
			return excludes, nil
		}
	}
	return nil, nil
}
