// Run with: npm test
import { test } from "node:test";
import assert from "node:assert/strict";

import { parseTime, timeAgo } from "./time.js";

// A time the given number of seconds ago, in the API's SQLite format.
function secondsAgo(seconds) {
  return new Date(Date.now() - seconds * 1000).toISOString().slice(0, 19).replace("T", " ");
}

test("parseTime reads the API's UTC format", () => {
  assert.equal(parseTime("2026-10-13 21:35:45").toISOString(), "2026-10-13T21:35:45.000Z");
});

test("timeAgo picks the right unit", () => {
  assert.equal(timeAgo(secondsAgo(5)), "just now");
  assert.equal(timeAgo(secondsAgo(5 * 60)), "5 min ago");
  assert.equal(timeAgo(secondsAgo(3 * 3600)), "3 h ago");
  assert.equal(timeAgo(secondsAgo(2 * 86400)), "2 d ago");
});

test("timeAgo shows a date after a week", () => {
  const s = secondsAgo(10 * 86400);
  assert.equal(timeAgo(s), parseTime(s).toLocaleDateString());
});

test("timeAgo treats a time slightly in the future as just now", () => {
  // The server clock can be a little ahead of the browser's.
  assert.equal(timeAgo(secondsAgo(-30)), "just now");
});
