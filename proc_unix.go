//go:build !windows

package main

import "os/exec"

func applyProcAttr(cmd *exec.Cmd) {}
