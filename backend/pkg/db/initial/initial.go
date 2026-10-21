// Package initial ships the starting database and the images it points to,
// so a fresh clone opens with the same users, posts and chats.
package initial

import (
	"embed"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

//go:embed social-network.db uploads
var files embed.FS

// Install copies the starting database to dbPath when no database is there
// yet, and each of its images to uploadDir when that file is missing.
// Existing files are never overwritten.
func Install(dbPath, uploadDir string) error {
	if err := copyIfMissing("social-network.db", dbPath); err != nil {
		return err
	}
	images, err := files.ReadDir("uploads")
	if err != nil {
		return err
	}
	for _, img := range images {
		if err := copyIfMissing(path.Join("uploads", img.Name()), filepath.Join(uploadDir, img.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyIfMissing(name, dest string) error {
	if _, err := os.Stat(dest); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	data, err := files.ReadFile(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}
