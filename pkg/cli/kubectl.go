package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

type KubeCtl struct {
	cli *CLI
}

func NewKubeCtl(cli *CLI) *KubeCtl {
	return &KubeCtl{cli}
}

func (ctl *KubeCtl) GetPodName(selector string) (string, error) {
	cmd := ctl.cli.Command(context.Background(), "get", "pods", "--field-selector=status.phase==Running", "--selector", selector, "-o", "jsonpath='{.items[0].metadata.name}'")
	stdout := bytes.Buffer{}
	cmd.Stdout = &stdout

	cmd.Run()

	name := strings.Trim(strings.Trim(stdout.String(), ""), "'")
	if len(name) == 0 {
		return "", fmt.Errorf("not found pod by selector \"%s\"", selector)
	}

	return name, nil
}
