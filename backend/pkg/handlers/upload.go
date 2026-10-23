package handlers

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const (
	maxImageBytes = 50 << 20
	maxFormBytes  = 64 << 20
)

var imageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

type uploadError string

func (e uploadError) Error() string { return string(e) }

const (
	errImageTooLarge = uploadError("image must be 50 MB or smaller")
	errImageType     = uploadError("image must be a JPEG, PNG, GIF or WebP")
)

func parseForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	err := r.ParseMultipartForm(1 << 20)
	if err == nil || errors.Is(err, http.ErrNotMultipart) {
		return true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusBadRequest, string(errImageTooLarge))
		return false
	}
	writeError(w, http.StatusBadRequest, "invalid form data")
	return false
}

func removeFormFiles(r *http.Request) {
	if r.MultipartForm != nil {
		r.MultipartForm.RemoveAll()
	}
}

func (a *App) saveImage(r *http.Request, field string) (string, error) {
	file, header, err := r.FormFile(field)
	if errors.Is(err, http.ErrMissingFile) || errors.Is(err, http.ErrNotMultipart) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer file.Close()

	if header.Size > maxImageBytes {
		return "", errImageTooLarge
	}

	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	head = head[:n]
	ext, ok := imageExtensions[http.DetectContentType(head)]
	if !ok {
		return "", errImageType
	}

	name := uuid.NewString() + ext
	dst, err := os.OpenFile(filepath.Join(a.UploadDir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	written, err := io.Copy(dst, io.LimitReader(io.MultiReader(bytes.NewReader(head), file), maxImageBytes+1))
	if closeErr := dst.Close(); err == nil {
		err = closeErr
	}
	if err == nil && written > maxImageBytes {
		err = errImageTooLarge
	}
	if err != nil {
		os.Remove(filepath.Join(a.UploadDir, name))
		return "", err
	}
	return "/uploads/" + name, nil
}

func (a *App) removeImage(path string) {
	name := strings.TrimPrefix(path, "/uploads/")
	if name == "" || name == path || strings.ContainsAny(name, `/\`) {
		return
	}
	os.Remove(filepath.Join(a.UploadDir, name))
}
