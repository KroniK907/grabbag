package quips

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed shipped/*.json
var shippedFiles embed.FS

func copyShippedLibraries(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	entries, err := fs.ReadDir(shippedFiles, "shipped")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dest := filepath.Join(dir, entry.Name())
		if _, err := os.Stat(dest); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		raw, err := shippedFiles.ReadFile("shipped/" + entry.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, raw, 0o600); err != nil {
			return err
		}
	}
	return nil
}
