package main

import "os/exec"

func setProcessGroup(*exec.Cmd) {}

func killProcessGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}

func terminateProcessGroup(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
