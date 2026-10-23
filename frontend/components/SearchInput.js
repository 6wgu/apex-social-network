"use client";

import { SEARCH_MAX_LENGTH, fit, oneLine } from "@/lib/search";

export default function SearchInput({ value, onChange, ...props }) {
  function paste(e) {
    const text = e.clipboardData.getData("text");
    if (!text) return;
    e.preventDefault();
    const input = e.currentTarget;
    const start = input.selectionStart ?? value.length;
    const end = input.selectionEnd ?? value.length;
    const room = SEARCH_MAX_LENGTH - (value.length - (end - start));
    const pasted = fit(oneLine(text).trim(), room);
    onChange(value.slice(0, start) + pasted + value.slice(end));
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
