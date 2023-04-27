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
	if _, err := os.Stat(rootDestPath); !os.IsNotExist(err) {
		return fmt.Errorf("unable write simple store template if dir already exists")
	}

	if err := os.MkdirAll(rootDestPath, 0o700); err != nil {
		return fmt.Errorf("unable to create root dir for store: %v", err)
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
