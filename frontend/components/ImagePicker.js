"use client";

import { useEffect, useRef, useState } from "react";
import { IMAGE_ACCEPT, imageProblem } from "@/lib/images";

// "Add image / GIF" button with a preview and a remove button. The parent
// owns the File (value / onChange); reset it to null to clear the picker.
// A file the backend would refuse is not taken: onError gets the reason
// (and "" once a good file is chosen), for the form's own error line.
export default function ImagePicker({ value, onChange, onError, label = "Photo / GIF", compact = false }) {
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
        accept={IMAGE_ACCEPT}
        hidden
        onChange={(e) => {
          const file = e.target.files[0] || null;
          const problem = file ? imageProblem(file) : "";
          onError?.(problem ? `Image ${problem}` : "");
          if (problem) e.target.value = "";
          else onChange(file);
        }}
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
