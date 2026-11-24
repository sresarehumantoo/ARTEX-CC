package api

import "os/exec"

func setProcessGroup(*exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}

// Windows has no SIGINT for child processes, so stopping a run kills it.
func interruptProcessGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
