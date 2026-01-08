package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	// Check sudo is available non-interactively
	if err := exec.Command("sudo", "-n", "true").Run(); err != nil {
		fmt.Fprintln(os.Stderr, "sudo not available non-interactively (requires password or not installed)")
		os.Exit(2)
	}

	// List allowed commands for current user
	out, err := exec.Command("sudo", "-n", "-l").CombinedOutput()
	if err != nil {
		// Some sudoers configurations may still allow running specific commands
		fmt.Fprintln(os.Stderr, "failed to list sudo privileges:", err)
		os.Exit(3)
	}

	s := string(out)
	if strings.Contains(s, "chpasswd") {
		fmt.Println("sudo can run chpasswd (non-interactive): OK")
		os.Exit(0)
	}

	fmt.Println("sudo is available non-interactively, but chpasswd was not found in 'sudo -l' output. Please add a sudoers rule to allow /usr/sbin/chpasswd as described in docs/SUDO_PASSWORD_CHANGE.md")
	os.Exit(1)
}
