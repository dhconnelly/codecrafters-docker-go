package main

import (
	"errors"
	"log"
	"os"
	"os/exec"
	"syscall"
)

func parent(args parentArgs) {
	log.Println("authenticating to docker hub...")
	client, err := newDockerClient(args.img)
	if err != nil {
		log.Fatalf("failed to connect to docker hub: %s", err)
	}

	log.Printf("fetching manifest for %s...\n", args.img)
	manifest, err := client.fetchManifest(args.img)
	if err != nil {
		log.Fatalf("failed to fetch manifest: %s", err)
	}

	chrootDir, err := os.MkdirTemp("", "mydocker")
	if err != nil {
		log.Fatalf("failed to create chroot dir: %s", err)
	}

	for _, layer := range manifest.Layers {
		log.Printf("fetching layer %s...", layer.Digest)
		path, err := client.downloadLayer(args.img, layer.Digest, chrootDir)
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

	cmd := exec.Command(
		"/proc/self/exe",
		append([]string{"container", chrootDir, args.command}, args.args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// set up the container
	// TODO: cgroups for resource management
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Unshareflags: syscall.CLONE_NEWNS, // new pid namespace
		Cloneflags: syscall.CLONE_NEWPID | // pid 1
			syscall.CLONE_NEWNS | // new mount namespace
			syscall.CLONE_NEWUTS, // new hostname namespace
	}

	log.Println("spawning containerized child process...")
	var exitError *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exitError) {
		os.Exit(exitError.ExitCode())
	} else if err != nil {
		log.Fatalf("failed to spawn child process: %s", err)
	}
}

func child(args childArgs) {
	log.Println("child entering chroot...")
	if err := syscall.Chroot(args.chroot); err != nil {
		log.Fatalf("failed to chroot: %s", err)
	}
	if err := os.Chdir("/"); err != nil {
		log.Fatalf("failed to enter chroot dir: %s", err)
	}

	cmd := exec.Command(args.command, args.args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("child executing %s %s in container...", args.command, args.args)
	var exitError *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exitError) {
		os.Exit(exitError.ExitCode())
	} else if err != nil {
		log.Fatalf("failed to spawn child process: %s", err)
	}
}

// Shout out to Liz Rice:
// https://www.youtube.com/watch?v=8fi7uSYlOdc
func main() {
	proc := os.Args[1]
	switch proc {
	case "run":
		parent(parseParentArgs(os.Args[2:]))
	case "container":
		child(parseChildArgs(os.Args[2:]))
	default:
		log.Fatalf("invalid process: %s\n", proc)
	}
}
