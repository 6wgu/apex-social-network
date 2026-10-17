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
	// maxImageBytes is the largest image we store.
	maxImageBytes = 5 << 20
	// maxFormBytes is the hard cap on a multipart request. It is well above
	// maxImageBytes on purpose: a request under this cap is read to the end,
	// so a too-big image gets a proper 400. Cutting the connection right at
	// 5 MB makes clients and the Next.js proxy report a network error instead.
	maxFormBytes = 32 << 20
)

// imageExtensions maps the content types we accept to the extension the file
// is saved with. The type comes from the file's bytes, never from its name.
var imageExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// uploadError is a problem with the uploaded file that the client can fix.
// Its text is safe to send back as a 400.
type uploadError string

func (e uploadError) Error() string { return string(e) }

const (
	errImageTooLarge = uploadError("image must be 5 MB or smaller")
	errImageType     = uploadError("image must be a JPEG, PNG, GIF or WebP")
)

// parseForm reads a multipart (or URL-encoded) form with the body capped at
// maxFormBytes. Parts over 1 MB go to temporary files, so the caller must
// defer removeFormFiles(r); saveImage enforces the per-image limit. It writes
// the 400 itself and returns false when the form cannot be used.
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

// removeFormFiles deletes the temporary files ParseMultipartForm created.
// Handlers that call parseForm defer it. net/http only cleans up the request
// it created, and middleware that adds to the context (RequireAuth) hands the
// handler a copy, so without this the temp files would pile up.
func removeFormFiles(r *http.Request) {
	if r.MultipartForm != nil {
		r.MultipartForm.RemoveAll()
	}
}

// saveImage stores the file uploaded in the given form field and returns the
// path it is served from, e.g. "/uploads/3f2a….png". It returns "" when the
// field has no file, and an uploadError when the file is too big or is not a
// JPEG, PNG, GIF or WebP. Call parseForm first.
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

	// DetectContentType looks at no more than the first 512 bytes.
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
	// Copy one byte past the limit so an oversized file is noticed even if
	// the size in the multipart header was wrong.
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

// removeImage deletes a stored image given the path saveImage returned.
// It ignores "" and anything that is not a plain file name under /uploads/.
func (a *App) removeImage(path string) {
	name := strings.TrimPrefix(path, "/uploads/")
	if name == "" || name == path || strings.ContainsAny(name, `/\`) {
		return
	}
	os.Remove(filepath.Join(a.UploadDir, name))
}
