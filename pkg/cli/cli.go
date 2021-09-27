package cli

import (
	"context"
	"os/exec"
)

type CLI struct {
	kubeContext string
	namespace   string
}

func NewCLI(kubeContext, namespace string) *CLI {
	return &CLI{
		kubeContext: kubeContext,
		namespace:   namespace,
	}
}

func (c *CLI) Command(ctx context.Context, command string, arg ...string) *exec.Cmd {
	args := c.args(command, arg...)
	return exec.CommandContext(ctx, "kubectl", args...)
}

func (c *CLI) args(command string, arg ...string) []string {
	args := []string{}
	if c.kubeContext != "" {
		args = append(args, "--context", c.kubeContext)
	}
	if c.namespace != "" {
		args = append(args, "--namespace", c.namespace)
	}

	args = append(args, command)
	args = append(args, arg...)
	return args
}
