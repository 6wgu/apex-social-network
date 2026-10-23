import { test } from "node:test";
import assert from "node:assert/strict";

import { IMAGE_ACCEPT, MAX_IMAGE_MB, imageProblem } from "./images.js";

const MB = 1024 * 1024;
const file = (name, type, size = 1000) => ({ name, type, size });

test("JPEG, PNG, GIF and WebP files pass", () => {
  for (const [name, type] of [
    ["a.jpg", "image/jpeg"],
    ["a.JPEG", "image/jpeg"],
    ["a.png", "image/png"],
    ["a.gif", "image/gif"],
    ["a.webp", "image/webp"],
  ]) {
    assert.equal(imageProblem(file(name, type)), "", name);
  }
});

test("other files are refused by type or by name", () => {
  const wrong = "must be a JPEG, PNG, GIF or WebP file";
  assert.equal(imageProblem(file("notes.txt", "text/plain")), wrong);
  assert.equal(imageProblem(file("photo.svg", "image/svg+xml")), wrong);
  assert.equal(imageProblem(file("photo.png.exe", "image/png")), wrong);
  assert.equal(imageProblem(file("photo.png", "")), wrong);
});

test("the size limit matches the backend's", () => {
  assert.equal(MAX_IMAGE_MB, 50);
  assert.equal(imageProblem(file("big.gif", "image/gif", 50 * MB)), "");
  assert.equal(imageProblem(file("big.gif", "image/gif", 50 * MB + 1)), "must be 50 MB or smaller");
});

test("the accept list names every allowed type and extension", () => {
  for (const part of ["image/jpeg", "image/png", "image/gif", "image/webp", ".jpg", ".jpeg", ".png", ".gif", ".webp"]) {
    assert.ok(IMAGE_ACCEPT.split(",").includes(part), part);
  }
});
