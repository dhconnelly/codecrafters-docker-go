package main

import (
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	// Uncomment this block to pass the first stage!
	// "os"
	// "os/exec"
)

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	// set up chroot directory
	dir, err := os.MkdirTemp("", "mydocker")
	if err != nil {
		log.Fatalf("failed to create chroot dir: %s", err)
	}

	// copy the binary to the chroot
	originalBin, err := os.Open(command)
	if err != nil {
		log.Fatalf("failed to open original binary: %s", err)
	}
	originalStat, err := originalBin.Stat()
	if err != nil {
		log.Fatalf("failed to stat original binary: %s", err)
	}
	relCommand := filepath.Base(command)
	targetBin, err := os.OpenFile(
		path.Join(dir, relCommand),
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		originalStat.Mode())
	if err != nil {
		log.Fatalf("failed to create new target binary: %s", err)
	}
	if _, err = io.Copy(targetBin, originalBin); err != nil {
		log.Fatalf("failed to copy binary: %s", err)
	}
	originalBin.Close()
	targetBin.Close()

	// chroot
	syscall.Chroot(dir)

	// run the program in the chroot
	chrootCommand := filepath.Join("/", relCommand)
	cmd := exec.Command(chrootCommand, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// exit
	var exitError *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exitError) {
		os.Exit(exitError.ExitCode())
	} else if err != nil {
		log.Fatalf("failed to run %s in chroot dir %s: %s", chrootCommand, dir, err)
	}
}
