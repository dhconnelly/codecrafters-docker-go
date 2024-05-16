package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
