// The image rules every upload follows (avatars, posts and comments), the
// same ones the backend enforces (backend/pkg/handlers/upload.go). The
// server checks the file's bytes again; these let a form refuse a wrong
// file before it is sent.

// For the file input's accept attribute.
export const IMAGE_ACCEPT = "image/jpeg,image/png,image/gif,image/webp,.jpg,.jpeg,.png,.gif,.webp";

const IMAGE_TYPES = new Set(["image/jpeg", "image/png", "image/gif", "image/webp"]);

// What is wrong with a chosen file, worded to follow its name ("avatar
// must be …"), or "" when it is fine. A JPEG, PNG, GIF or WebP by both its
// type and its file name passes.
export function imageProblem(file) {
  if (!IMAGE_TYPES.has(file.type) || !/\.(jpe?g|png|gif|webp)$/i.test(file.name)) {
    return "must be a JPEG, PNG, GIF or WebP file";
  }
  return "";
}
