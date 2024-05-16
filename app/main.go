package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/codeclysm/extract"
)

type AuthResponse struct {
	Token string `json:"token"`
}

type Layer struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type Manifest struct {
	MediaType string  `json:"mediaType"`
	Layers    []Layer `json:"layers"`
}

func copyFile(toPath, fromPath string) error {
	fromF, err := os.Open(fromPath)
	if err != nil {
		return err
	}
	defer fromF.Close()
	originalStat, err := fromF.Stat()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(toPath), 0644); err != nil {
		return err
	}
	toF, err := os.OpenFile(
		toPath,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		originalStat.Mode())
	if err != nil {
		return err
	}
	defer toF.Close()
	_, err = io.Copy(toF, fromF)
	return err
}

func stripExtensions(path string) string {
	for {
		ext := filepath.Ext(path)
		if len(ext) == 0 {
			break
		}
		cut, ok := strings.CutSuffix(path, ext)
		if !ok {
			break
		}
		path = cut
	}
	return path
}

type dockerExtractor struct{}

func (dockerExtractor) Link(oldname, newname string) error {
	if _, err := os.Lstat(newname); err == nil {
		return os.Remove(newname)
	}
	return os.Link(oldname, newname)
}

func (dockerExtractor) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}

func (dockerExtractor) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (dockerExtractor) Symlink(oldname, newname string) error {
	if _, err := os.Lstat(newname); err == nil {
		return os.Remove(newname)
	}
	return os.Symlink(oldname, newname)
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

	// fetch the manifest
	manifestURL := fmt.Sprintf("https://registry-1.docker.io/v2/library/%s/manifests/%s", name, tag)
	manifestReq, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		log.Fatalf("failed to make manifest request: %s", err)
	}
	manifestReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	manifestReq.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	manifestResp, err := http.DefaultClient.Do(manifestReq)
	if err != nil {
		log.Fatalf("failed to fetch manifest: %s", err)
	}
	manifestDecoder := json.NewDecoder(manifestResp.Body)
	var manifest Manifest
	if err = manifestDecoder.Decode(&manifest); err != nil {
		log.Fatalf("failed to decode manifest response: %s", err)
	}
	if manifest.MediaType != "application/vnd.docker.distribution.manifest.v2+json" {
		log.Fatalf("only docker manifest v2 supported for now, found %s", manifest.MediaType)
	}

	// set up chroot directory
	dir, err := os.MkdirTemp("", "mydocker")
	if err != nil {
		log.Fatalf("failed to create chroot dir: %s", err)
	}

	// fetch layers
	var layerPaths []string
	for _, layer := range manifest.Layers {
		layerURL := fmt.Sprintf(
			"https://registry-1.docker.io/v2/library/%s/blobs/%s",
			name, layer.Digest)
		layerReq, err := http.NewRequest("GET", layerURL, nil)
		if err != nil {
			log.Fatalf("failed to make layer request: %s", err)
		}
		layerReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
		layerResp, err := http.DefaultClient.Do(layerReq)
		if err != nil {
			log.Fatalf("failed to fetch layer: %s", err)
		}
		f, err := os.CreateTemp(dir, "layer-*.tar.gz")
		if err != nil {
			log.Fatalf("failed to create layer file: %s", err)
		}
		if _, err = io.Copy(f, layerResp.Body); err != nil {
			log.Fatalf("failed to download layer file: %s", err)
		}
		layerPaths = append(layerPaths, f.Name())
		f.Close()
	}

	// extract each layer in order
	extractor := extract.Extractor{FS: dockerExtractor{}}
	for _, layerPath := range layerPaths {
		f, err := os.Open(layerPath)
		if err != nil {
			log.Fatalf("failed to open layer: %s", err)
		}
		if err = extractor.Gz(context.Background(), f, dir, nil); err != nil {
			log.Fatalf("failed to gunzip layer: %s", err)
		}
		f.Close()
		if err = os.Remove(layerPath); err != nil {
			log.Fatalf("failed to remove layer file: %s", err)
		}
	}

	command := os.Args[3]
	args := os.Args[4:len(os.Args)]

	// run the program in the chroot in a new namespace
	chrootCommand := filepath.Join("/", command)
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
