"use client";

import { useState } from "react";
import Icon from "@/components/Icon";

// A labelled form field with an icon inside the input's left edge.
// Pass the <input>/<textarea> as children; extra hint text goes in `hint`.
export default function Field({ label, icon, hint, className = "", children, trailing }) {
  return (
    <label className={`field ${className}`.trim()}>
      {label && <span className="field-label">{label}</span>}
      <span className={`field-box${icon ? " has-icon" : ""}${trailing ? " has-trailing" : ""}`}>
        {icon && <Icon name={icon} size={17} className="field-icon" />}
        {children}
        {trailing}
      </span>
      {hint && <span className="hint">{hint}</span>}
    </label>
  );
}

// A password input with a show / hide button. Everything else is passed to <input>.
export function PasswordField({ label = "Password", hint, ...input }) {
  const [shown, setShown] = useState(false);
  return (
    <Field
      label={label}
      icon="lock"
      hint={hint}
      trailing={
        <button
          type="button"
          className="field-eye"
          onClick={() => setShown((s) => !s)}
          aria-label={shown ? "Hide password" : "Show password"}
          aria-pressed={shown}
        >
          <Icon name={shown ? "eyeOff" : "eye"} size={17} />
        </button>
      }
    >
      <input {...input} type={shown ? "text" : "password"} />
    </Field>
  );
}
