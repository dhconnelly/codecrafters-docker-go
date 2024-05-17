package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"syscall"
)

func main() {
	args, ok := parseArgs(os.Args)
	if !ok {
		log.Fatalf("usage: your_docker.sh run <image> <command> <args...>")
	}

	log.Println("authenticating to docker hub...")
	client, err := newDockerClient(args.imageName)
	if err != nil {
		log.Fatalf("failed to connect to docker hub: %s", err)
	}

	log.Printf("fetching manifest for %s:%s...\n", args.imageName, args.imageTag)
	manifest, err := client.fetchManifest(args.imageName, args.imageTag)
	if err != nil {
		log.Fatalf("failed to fetch manifest: %s", err)
	}

	chrootDir, err := os.MkdirTemp("", "mydocker")
	if err != nil {
		log.Fatalf("failed to create chroot dir: %s", err)
	}

	for _, layer := range manifest.Layers {
		log.Printf("fetching layer %s...", layer.Digest)
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

	log.Printf("executing %s %s in container...", args.command, args.args)
	if err = os.Chdir(chrootDir); err != nil {
		log.Fatalf("failed to enter chroot dir: %s", err)
	}
	cmd := exec.Command(args.command, args.args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// set up the container
	// https://ericchiang.github.io/post/containers-from-scratch/
	// https://www.youtube.com/watch?v=8fi7uSYlOdc
	// TODO: cgroups for resource management
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:       chrootDir,           // new root
		Unshareflags: syscall.CLONE_NEWNS, // new pid namespace
		Cloneflags: syscall.CLONE_NEWPID | // pid 1
			syscall.CLONE_NEWNS | // new mount namespace
			syscall.CLONE_NEWUTS, // new hostname namespace
	}

	var exitError *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exitError) {
		os.Exit(exitError.ExitCode())
	} else if err != nil {
		log.Fatalf("failed to run %s in container: %s", args.command, err)
	}
}
