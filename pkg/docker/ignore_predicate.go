package docker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/pkg/fileutils"
)

type Dirent interface {
	IsDir() bool
	Name() string
}

type Predicate func(path string, info Dirent) (bool, error)

func NewDockerIgnorePredicate(workspace string, excludes []string) (Predicate, error) {
	matcher, err := fileutils.NewPatternMatcher(excludes)
	if err != nil {
		return nil, fmt.Errorf("invalid exclude patterns: %w", err)
	}

	skipDir := func(relPath string, matcher *fileutils.PatternMatcher) bool {
		// No exceptions (!...) in patterns so just skip dir
		if !matcher.Exclusions() {
			return true
		}

		dirSlash := relPath + string(filepath.Separator)

		for _, pat := range matcher.Patterns() {
			if !pat.Exclusion() {
				continue
			}
			if strings.HasPrefix(pat.String()+string(filepath.Separator), dirSlash) {
				// found a match - so can't skip this dir
				return false
			}
		}

		return true
	}

	return func(path string, info Dirent) (bool, error) {
		relPath, err := filepath.Rel(workspace, path)
		if err != nil {
			return false, err
		}

		ignored, err := matcher.Matches(relPath)
		if err != nil {
			return false, err
		}

		if ignored && (info != nil && info.IsDir()) && skipDir(relPath, matcher) {
			return false, filepath.SkipDir
		}

		return ignored, nil
	}, nil
}
