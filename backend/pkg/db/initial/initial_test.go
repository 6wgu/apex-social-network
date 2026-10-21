package initial

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallFillsOnlyWhatIsMissing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data", "social-network.db")
	uploads := filepath.Join(dir, "uploads")

	if err := Install(dbPath, uploads); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(dbPath); err != nil || info.Size() == 0 {
		t.Fatalf("database not installed: %v", err)
	}
	images, _ := files.ReadDir("uploads")
	if len(images) == 0 {
		t.Fatal("no images shipped")
	}
	for _, img := range images {
		if _, err := os.Stat(filepath.Join(uploads, img.Name())); err != nil {
			t.Errorf("image %s not installed: %v", img.Name(), err)
		}
	}

	// An existing database (and image) is left exactly as it is.
	os.WriteFile(dbPath, []byte("mine"), 0o644)
	mine := filepath.Join(uploads, images[0].Name())
	os.WriteFile(mine, []byte("mine too"), 0o644)
	if err := Install(dbPath, uploads); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dbPath); string(got) != "mine" {
		t.Error("existing database was overwritten")
	}
	if got, _ := os.ReadFile(mine); string(got) != "mine too" {
		t.Error("existing image was overwritten")
	}
}
