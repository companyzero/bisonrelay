package simplestore

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed template
var storeTemplate embed.FS

// WriteTemplate writes the store template in the given path.
func WriteTemplate(rootDestPath string) error {
	if err := os.MkdirAll(rootDestPath, 0o700); err != nil {
		return fmt.Errorf("unable to create root dir for store: %w", err)
	}

	if entries, err := os.ReadDir(rootDestPath); err != nil {
		return fmt.Errorf("failed to read %v: %w", rootDestPath, err)
	} else if len(entries) != 0 {
		return os.ErrExist
	}

	walkDirFunc := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Type().IsRegular() {
			data, err := storeTemplate.ReadFile(path)
			if err != nil {
				return fmt.Errorf("unable to read template file %s: %v",
					path, err)
			}

			// path[8:] skips the 'template/' prefix.
			destFilename := filepath.Join(rootDestPath, path[9:])
			if err := os.WriteFile(destFilename, data, 0o644); err != nil {
				return fmt.Errorf("unable to write template file %s: %v",
					destFilename, err)
			}

			return nil
		}

		if d.IsDir() && path != "template" {
			// path[8:] skips the 'template/' prefix.
			destFilename := filepath.Join(rootDestPath, path[9:])
			return os.Mkdir(destFilename, 0o700)
		}

		return nil
	}

	return fs.WalkDir(storeTemplate, "template", walkDirFunc)
}
