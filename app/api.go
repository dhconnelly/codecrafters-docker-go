package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const manifestMediaType string = "application/vnd.docker.distribution.manifest.v2+json"

type image struct {
	repo string
	name string
	tag  string
}

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

// TODO: make a client struct to wrap the token
type dockerClient struct {
	token string
}

func newDockerClient(img image) (*dockerClient, error) {
	token, err := authenticate(img)
	if err != nil {
		return nil, err
	}
	return &dockerClient{token: token}, nil
}

func authenticate(img image) (string, error) {
	// TODO: extract base url
	// TODO: extract constants
	authURL := fmt.Sprintf(
		"https://auth.docker.io/token?client_id=dhcdocker&service=registry.docker.io&scope=repository:%s/%s:pull",
		img.repo,
		img.name)
	authResp, err := http.Get(authURL)
	if err != nil {
		return "", err
	}
	tokenDecoder := json.NewDecoder(authResp.Body)
	var auth AuthResponse
	if err = tokenDecoder.Decode(&auth); err != nil {
		return "", err
	}
	return auth.Token, nil
}

func (client *dockerClient) fetchManifest(img image) (Manifest, error) {
	manifestURL := fmt.Sprintf("https://registry-1.docker.io/v2/%s/%s/manifests/%s", img.repo, img.name, img.tag)
	manifestReq, err := http.NewRequest("GET", manifestURL, nil)
	if err != nil {
		return Manifest{}, err
	}
	manifestReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", client.token))
	manifestReq.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	manifestResp, err := http.DefaultClient.Do(manifestReq)
	if err != nil {
		return Manifest{}, err
	}
	manifestDecoder := json.NewDecoder(manifestResp.Body)
	var manifest Manifest
	if err = manifestDecoder.Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.MediaType != manifestMediaType {
		return Manifest{}, fmt.Errorf(
			"unsupported media type: want %s, got %s",
			manifestMediaType,
			manifest.MediaType)
	}
	return manifest, nil
}

func (client *dockerClient) fetchLayer(w io.Writer, img image, digest string) error {
	layerURL := fmt.Sprintf(
		"https://registry-1.docker.io/v2/%s/%s/blobs/%s",
		img.repo, img.name, digest)
	layerReq, err := http.NewRequest("GET", layerURL, nil)
	if err != nil {
		return err
	}
	layerReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", client.token))
	layerResp, err := http.DefaultClient.Do(layerReq)
	if err != nil {
		return err
	}
	if _, err = io.Copy(w, layerResp.Body); err != nil {
		return err
	}
	return nil
}

func (client *dockerClient) downloadLayer(img image, digest, dir string) (string, error) {
	f, err := os.CreateTemp(dir, "layer-*.tar.gz")
	if err != nil {
		return "", err
	}
	if err = client.fetchLayer(f, img, digest); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}
