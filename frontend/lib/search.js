export const SEARCH_MAX_LENGTH = 50;

export const SEARCH_DELAY_MS = 300;

const ECHO_LENGTH = 30;

export function oneLine(text) {
  return text.replace(/\s+/g, " ");
}

export function fit(text, max) {
  if (text.length <= max) return text;
  const cut = text.slice(0, Math.max(0, max));
  return /[\uD800-\uDBFF]$/.test(cut) ? cut.slice(0, -1) : cut;
}

export function searchQuery(text) {
  return fit(oneLine(text).trim(), SEARCH_MAX_LENGTH);
}

export function echoQuery(query) {
  const chars = [...query];
  return chars.length > ECHO_LENGTH ? `${chars.slice(0, ECHO_LENGTH).join("")}…` : query;
}
