package cli

import (
	"bytes"
	"context"
	"strings"
)

type KubeCtl struct {
	cli *CLI
}

func NewKubeCtl(cli *CLI) *KubeCtl {
	return &KubeCtl{cli}
}

func (ctl *KubeCtl) GetPodName(selector string) (string, error) {
	cmd := ctl.cli.Command(context.Background(), "get", "pods", "--selector", selector, "-o", "jsonpath='{.items[0].metadata.name}'")
	stdout := bytes.Buffer{}
	cmd.Stdout = &stdout

	cmd.Run()

	return strings.Trim(stdout.String(), "'"), nil
}
