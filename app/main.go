package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	args, ok := parseArgs(os.Args)
	if !ok {
		log.Fatalf("usage: your_docker.sh run <image> <command> <args...>")
	}

	client, err := newDockerClient(args.imageName)
	if err != nil {
		log.Fatalf("failed to connect to docker hub: %s", err)
	}

	manifest, err := client.fetchManifest(args.imageName, args.imageTag)
	if err != nil {
		log.Fatalf("failed to fetch manifest: %s", err)
	}

	chrootDir, err := os.MkdirTemp("", "mydocker")
	if err != nil {
		log.Fatalf("failed to create chroot dir: %s", err)
	}

	for _, layer := range manifest.Layers {
		path, err := client.downloadLayer(args.imageName, layer.Digest, chrootDir)
		if err != nil {
			log.Fatalf("failed to download layer: %s", err)
		}
		if err = extractInto(chrootDir, path); err != nil {
			log.Fatalf("failed to extract layer: %s", err)
		}
		if err = os.Remove(path); err != nil {
			log.Fatalf("failed to clean up layer: %s", err)
		}
	}

	chrootCommand := filepath.Join("/", args.command)
	cmd := exec.Command(chrootCommand, args.args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:     chrootDir,
		Cloneflags: syscall.CLONE_NEWPID,
	}

	var exitError *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exitError) {
		os.Exit(exitError.ExitCode())
	} else if err != nil {
		log.Fatalf("failed to run %s in chroot with newpid: %s", chrootCommand, err)
	}
}
