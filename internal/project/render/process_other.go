//go:build !unix

package render

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}
