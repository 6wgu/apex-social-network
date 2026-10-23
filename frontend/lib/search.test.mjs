// Run with: npm test
import { test } from "node:test";
import assert from "node:assert/strict";

import { SEARCH_MAX_LENGTH, echoQuery, fit, oneLine, searchQuery } from "./search.js";

test("oneLine turns any whitespace run into one space", () => {
  assert.equal(oneLine("a\n\tb   c"), "a b c");
});

test("fit keeps text that already fits", () => {
  assert.equal(fit("abc", 3), "abc");
  assert.equal(fit("abcd", 3), "abc");
  assert.equal(fit("abc", 0), "");
});

test("fit never cuts an emoji in half", () => {
  // "🏎" is two UTF-16 units, so a cut after "a" plus one unit drops it.
  assert.equal(fit("a🏎b", 2), "a");
  assert.equal(fit("a🏎b", 3), "a🏎");
});

test("searchQuery is one trimmed line within the limit", () => {
  assert.equal(searchQuery("  ada \n lovelace  "), "ada lovelace");
  assert.equal(searchQuery("x".repeat(80)).length, SEARCH_MAX_LENGTH);
});

test("echoQuery shortens long queries with an ellipsis", () => {
  assert.equal(echoQuery("short"), "short");
  assert.equal(echoQuery("y".repeat(31)), `${"y".repeat(30)}…`);
  // Counted in characters, so emoji are not split.
  assert.equal(echoQuery("🏎".repeat(31)), `${"🏎".repeat(30)}…`);
});
