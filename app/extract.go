package main

import (
	"context"
	"os"

	"github.com/codeclysm/extract/v3"
)

type extractorFS struct{}

func (extractorFS) Link(oldname, newname string) error {
	if _, err := os.Lstat(newname); err == nil {
		return os.Remove(newname)
	}
	return os.Link(oldname, newname)
}

func (extractorFS) MkdirAll(path string, mode os.FileMode) error {
	return os.MkdirAll(path, mode)
}

func (extractorFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (extractorFS) Symlink(oldname, newname string) error {
	if _, err := os.Lstat(newname); err == nil {
		return os.Remove(newname)
	}
	return os.Symlink(oldname, newname)
}

func newExtractor() *extract.Extractor {
	return &extract.Extractor{FS: extractorFS{}}
}

func extractInto(dir, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return newExtractor().Gz(context.Background(), f, dir, nil)
}
