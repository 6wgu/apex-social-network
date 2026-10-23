package handlers

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// uploadRequest is a parsed multipart request with one file in the "image" field.
func uploadRequest(t *testing.T, filename string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("image", filename)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(content)
	mw.Close()

	r := httptest.NewRequest("POST", "/", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	if !parseForm(httptest.NewRecorder(), r) {
		t.Fatal("parseForm refused the form")
	}
	t.Cleanup(func() { removeFormFiles(r) })
	return r
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSaveImageNamesTheFileByItsBytes(t *testing.T) {
	a := &App{UploadDir: t.TempDir()}
	// A PNG uploaded under a misleading name is still saved as .png.
	path, err := a.saveImage(uploadRequest(t, "photo.txt", pngBytes(t)), "image")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, "/uploads/") || !strings.HasSuffix(path, ".png") {
		t.Fatalf("path = %q, want /uploads/<uuid>.png", path)
	}
	if _, err := os.Stat(filepath.Join(a.UploadDir, strings.TrimPrefix(path, "/uploads/"))); err != nil {
		t.Fatalf("saved file: %v", err)
	}
}

func TestSaveImageRejectsOtherFiles(t *testing.T) {
	a := &App{UploadDir: t.TempDir()}
	_, err := a.saveImage(uploadRequest(t, "evil.png", []byte("<script>alert(1)</script>")), "image")
	if !errors.Is(err, errImageType) {
		t.Fatalf("err = %v, want %v", err, errImageType)
	}
	if files, _ := os.ReadDir(a.UploadDir); len(files) != 0 {
		t.Fatalf("a rejected upload left %d file(s) behind", len(files))
	}
}

func TestSaveImageWithoutAFile(t *testing.T) {
	a := &App{UploadDir: t.TempDir()}
	r := uploadRequest(t, "a.png", pngBytes(t))
	if path, err := a.saveImage(r, "avatar"); path != "" || err != nil {
		t.Fatalf("saveImage(missing field) = %q, %v; want \"\", nil", path, err)
	}
}

func TestRemoveImageStaysInTheUploadFolder(t *testing.T) {
	root := t.TempDir()
	uploads := filepath.Join(root, "uploads")
	os.Mkdir(uploads, 0o755)
	outside := filepath.Join(root, "keep.txt")
	os.WriteFile(outside, []byte("x"), 0o644)
	inside := filepath.Join(uploads, "a.png")
	os.WriteFile(inside, []byte("x"), 0o644)

	a := &App{UploadDir: uploads}
	for _, p := range []string{"", "keep.txt", "/uploads/../keep.txt", `/uploads/..\keep.txt`, "/uploads/"} {
		a.removeImage(p)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("removeImage deleted a file outside the upload folder: %v", err)
	}

	a.removeImage("/uploads/a.png")
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("removeImage kept /uploads/a.png: %v", err)
	}
}
