"use client";

import { useEffect, useRef, useState } from "react";

// "Add image / GIF" button with a preview and a remove button. The parent
// owns the File (value / onChange); reset it to null to clear the picker.
export default function ImagePicker({ value, onChange, label = "Photo / GIF", compact = false }) {
  const input = useRef(null);
  const [preview, setPreview] = useState("");

  useEffect(() => {
    if (!value) {
      setPreview("");
      if (input.current) input.current.value = "";
      return;
    }
    const url = URL.createObjectURL(value);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [value]);

  return (
    <div className={`image-picker${compact ? " compact" : ""}`}>
      <input
        ref={input}
        type="file"
        accept="image/jpeg,image/png,image/gif"
        hidden
        onChange={(e) => onChange(e.target.files[0] || null)}
      />
      {!value && (
        <button type="button" className="btn-ghost" onClick={() => input.current.click()} aria-label={label}>
          🖼️ {compact ? "" : label}
        </button>
      )}
      {preview && (
        <div className="image-preview">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={preview} alt="Selected image" />
          <button type="button" className="image-remove" onClick={() => onChange(null)} aria-label="Remove image">
            ✕
          </button>
        </div>
      )}
    </div>
  );
}
