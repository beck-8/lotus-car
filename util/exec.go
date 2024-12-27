package util

import (
	"bytes"
	"fmt"
	"os/exec"
)

// ExecCmd 执行命令并返回输出
func ExecCmd(env, c string) (string, error) {
	cmd := exec.Command("sh", "-c", c)
	if env != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("FULLNODE_API_INFO=%s", env))
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
