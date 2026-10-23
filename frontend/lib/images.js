// The image rules every upload follows (avatars, posts and comments), the
// same ones the backend enforces (backend/pkg/handlers/upload.go). The
// server checks the file's bytes again; these let a form refuse a wrong
// file before it is sent.

// For the file input's accept attribute.
export const IMAGE_ACCEPT = "image/jpeg,image/png,image/gif,image/webp,.jpg,.jpeg,.png,.gif,.webp";

const IMAGE_TYPES = new Set(["image/jpeg", "image/png", "image/gif", "image/webp"]);

// The largest image the backend stores (maxImageBytes in upload.go). Change
// the two together.
export const MAX_IMAGE_MB = 50;
const MAX_IMAGE_BYTES = MAX_IMAGE_MB * 1024 * 1024;

// What is wrong with a chosen file, worded to follow its name ("avatar
// must be …"), or "" when it is fine. A JPEG, PNG, GIF or WebP by both its
// type and its file name, of at most MAX_IMAGE_MB, passes. Checking the size
// here saves sending up to 50 MB only for the server to refuse it.
export function imageProblem(file) {
  if (!IMAGE_TYPES.has(file.type) || !/\.(jpe?g|png|gif|webp)$/i.test(file.name)) {
    return "must be a JPEG, PNG, GIF or WebP file";
  }
  if (file.size > MAX_IMAGE_BYTES) return `must be ${MAX_IMAGE_MB} MB or smaller`;
  return "";
}
