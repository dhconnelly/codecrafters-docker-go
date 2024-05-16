package main

import (
	"os"
	"strings"
)

type invocation struct {
	imageName string
	imageTag  string
	command   string
	args      []string
}

func parseArgs(args []string) (*invocation, bool) {
	if len(args) < 4 {
		return nil, false
	}
	image := os.Args[2]
	toks := strings.Split(image, ":")
	if len(toks) < 1 {
		return nil, false
	}
	imageName := toks[0]
	imageTag := "latest"
	if len(toks) > 1 {
		imageTag = toks[1]
	}
	command := os.Args[3]
	rest := os.Args[4:len(os.Args)]
	return &invocation{
		imageName: imageName, imageTag: imageTag, command: command, args: rest,
	}, true
}
