"use client";

import { SEARCH_MAX_LENGTH, fit, oneLine } from "@/lib/search";

// A search or filter box: at most SEARCH_MAX_LENGTH characters, and pasted
// text arrives as one trimmed line (newlines and runs of spaces become one
// space), cut to what still fits, so a whole copied page never lands in it.
// onChange gets the new text, not the event; other props go to the <input>.
export default function SearchInput({ value, onChange, ...props }) {
  function paste(e) {
    const text = e.clipboardData.getData("text");
    if (!text) return; // nothing as text (an image): let the browser decide
    e.preventDefault();
    const input = e.currentTarget;
    const start = input.selectionStart ?? value.length;
    const end = input.selectionEnd ?? value.length;
    const room = SEARCH_MAX_LENGTH - (value.length - (end - start));
    const pasted = fit(oneLine(text).trim(), room);
    onChange(value.slice(0, start) + pasted + value.slice(end));
    // Leave the caret after the pasted text, as a normal paste does.
    const caret = start + pasted.length;
    requestAnimationFrame(() => input.setSelectionRange(caret, caret));
  }

  return (
    <input
      type="search"
      {...props}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onPaste={paste}
      maxLength={SEARCH_MAX_LENGTH}
    />
  );
}
