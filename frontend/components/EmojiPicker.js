"use client";

import { useEffect, useRef, useState } from "react";

// A small, built-in grid of common emojis (no extra package). Picking one
// calls onPick(emoji); the message box inserts it at the cursor.
const EMOJIS = [
  "😀", "😂", "🥲", "😊", "😍", "😘", "😎", "🤔", "😴", "😭",
  "😡", "😱", "🥳", "🤗", "🙄", "😅", "👍", "👎", "👏", "🙏",
  "💪", "👋", "🤝", "✌️", "👀", "❤️", "💔", "🔥", "✨", "🎉",
  "💯", "✅", "❌", "⭐", "☕", "🍕", "🎂", "⚽", "🏃", "🎵",
  "📷", "💬", "📅", "🏠", "🌙", "🌞", "🌧️", "👨‍👩‍👧", "🇸🇦", "🇧🇭",
];

export default function EmojiPicker({ onPick }) {
  const [open, setOpen] = useState(false);
  const wrapper = useRef(null);

  // Close on a click outside or on Escape.
  useEffect(() => {
    if (!open) return;
    const onClick = (e) => wrapper.current && !wrapper.current.contains(e.target) && setOpen(false);
    const onKey = (e) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", onClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div className="emoji-picker" ref={wrapper}>
      <button type="button" className="icon-btn" aria-label="Insert emoji" aria-expanded={open} onClick={() => setOpen(!open)}>
        😊
      </button>
      {open && (
        <div className="emoji-grid" role="listbox" aria-label="Emojis">
          {EMOJIS.map((e) => (
            <button
              key={e}
              type="button"
              role="option"
              aria-selected="false"
              onClick={() => onPick(e)}
              title={e}
            >
              {e}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
