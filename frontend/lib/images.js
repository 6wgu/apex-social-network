export const IMAGE_ACCEPT = "image/jpeg,image/png,image/gif,image/webp,.jpg,.jpeg,.png,.gif,.webp";

const IMAGE_TYPES = new Set(["image/jpeg", "image/png", "image/gif", "image/webp"]);

export const MAX_IMAGE_MB = 50;
const MAX_IMAGE_BYTES = MAX_IMAGE_MB * 1024 * 1024;

export function imageProblem(file) {
  if (!IMAGE_TYPES.has(file.type) || !/\.(jpe?g|png|gif|webp)$/i.test(file.name)) {
    return "must be a JPEG, PNG, GIF or WebP file";
  }
  if (file.size > MAX_IMAGE_BYTES) return `must be ${MAX_IMAGE_MB} MB or smaller`;
  return "";
}
