package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

type AuthResponse struct {
	Token string `json:"token"`
}

type FsLayer struct {
	BlobSum string `json:"blobSum"`
}

type ManifestResponse struct {
	FsLayers []FsLayer `json:"fsLayers"`
}

// Usage: your_docker.sh run <image> <command> <arg1> <arg2> ...
func main() {
	image := os.Args[2]
	toks := strings.Split(image, ":")
	name := toks[0]
	tag := toks[1]

	// authenticate
	authURL := fmt.Sprintf("https://auth.docker.io/token?client_id=dhcdocker&service=registry.docker.io&scope=repository:library/%s:pull", name)
	authResp, err := http.Get(authURL)
	if err != nil {
		log.Fatalf("failed to fetch auth token: %s", err)
	}
	tokenDecoder := json.NewDecoder(authResp.Body)
	var auth AuthResponse
	if err = tokenDecoder.Decode(&auth); err != nil {
		log.Fatalf("failed to decode auth response: %s", err)
	}
	token := auth.Token

	// fetch manifest
	manifestURL := fmt.Sprintf("https://registry-1.docker.io/v2/library/%s/manifests/%s", name, tag)
	manifestReq, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		log.Fatalf("failed to make manifest request: %s", err)
	}
	manifestReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	manifestResp, err := http.DefaultClient.Do(manifestReq)
	if err != nil {
		log.Fatalf("failed to fetch manifest: %s", err)
	}
	manifestDecoder := json.NewDecoder(manifestResp.Body)
	var manifest ManifestResponse
	if err = manifestDecoder.Decode(&manifest); err != nil {
		log.Fatalf("failed to decode manifest response: %s", err)
	}
	layers := manifest.FsLayers

	// set up chroot directory
	dir, err := os.MkdirTemp("", "mydocker")
	if err != nil {
		log.Fatalf("failed to create chroot dir: %s", err)
	}

	// fetch layers
	var layerPaths []string
	for _, layer := range layers {
		layerURL := fmt.Sprintf(
			"https://registry-1.docker.io/v2/library/%s/blobs/%s",
			name, layer.BlobSum)
		layerReq, err := http.NewRequest("GET", layerURL, nil)
		if err != nil {
			log.Fatalf("failed to make layer request: %s", err)
		}
		layerReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		layerResp, err := http.DefaultClient.Do(layerReq)
		if err != nil {
			log.Fatalf("failed to fetch layer: %s", err)
		}
		f, err := os.CreateTemp(dir, "layer")
		if err != nil {
			log.Fatalf("failed to create layer file: %s", err)
		}
		if _, err = io.Copy(f, layerResp.Body); err != nil {
			log.Fatalf("failed to download layer file: %s", err)
		}
		layerPaths = append(layerPaths, f.Name())
		f.Close()
	}

	fmt.Printf("saved layer files: %s\n", layerPaths)

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

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

	// run the program in the chroot in a new namespace
	chrootCommand := filepath.Join("/", relCommand)
	cmd := exec.Command(chrootCommand, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Chroot:     dir,
		Cloneflags: syscall.CLONE_NEWPID,
	}

	// exit
	var exitError *exec.ExitError
	if err := cmd.Run(); errors.As(err, &exitError) {
		os.Exit(exitError.ExitCode())
	} else if err != nil {
		log.Fatalf("failed to run %s in chroot dir %s: %s", chrootCommand, dir, err)
	}
}
