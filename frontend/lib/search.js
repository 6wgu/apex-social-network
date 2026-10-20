// The rules every search and filter box follows. The backend refuses a
// longer search (maxSearchLen in backend/pkg/handlers/profiles.go), so
// change the two together.
export const SEARCH_MAX_LENGTH = 50;

// How long typing must pause before a search runs.
export const SEARCH_DELAY_MS = 300;

// How much of a query a "Nobody matches …" message repeats.
const ECHO_LENGTH = 30;

// Search text is one line: newlines, tabs and runs of spaces become one space.
export function oneLine(text) {
  return text.replace(/\s+/g, " ");
}

// The first `max` UTF-16 units of text (what maxLength counts), without
// cutting an emoji in half.
export function fit(text, max) {
  if (text.length <= max) return text;
  const cut = text.slice(0, Math.max(0, max));
  return /[\uD800-\uDBFF]$/.test(cut) ? cut.slice(0, -1) : cut;
}

// What a search box searches for: one line, trimmed, within the limit.
export function searchQuery(text) {
  return fit(oneLine(text).trim(), SEARCH_MAX_LENGTH);
}

// A query to repeat back to the user: at most ECHO_LENGTH characters, then "…".
export function echoQuery(query) {
  const chars = [...query];
  return chars.length > ECHO_LENGTH ? `${chars.slice(0, ECHO_LENGTH).join("")}…` : query;
}
