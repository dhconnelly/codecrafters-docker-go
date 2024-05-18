package main

import (
	"strings"
)

type parentArgs struct {
	img     image
	command string
	args    []string
}

type childArgs struct {
	chroot  string
	command string
	args    []string
}

func parseImage(s string) image {
	img := image{repo: "library", tag: "latest"}
	slash := strings.Index(s, "/")
	colon := strings.Index(s, ":")
	if slash >= 0 {
		img.repo = s[:slash]
	}
	if colon >= 0 {
		img.tag = s[colon+1:]
	} else {
		colon = len(s)
	}
	img.name = s[slash+1 : colon]
	return img
}

func parseParentArgs(args []string) parentArgs {
	img := parseImage(args[0])
	command := args[1]
	rest := args[2:]
	return parentArgs{img: img, command: command, args: rest}
}

func parseChildArgs(args []string) childArgs {
	chroot := args[0]
	command := args[1]
	rest := args[2:]
	return childArgs{chroot: chroot, command: command, args: rest}
}
