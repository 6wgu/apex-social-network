"use client";

import { useState } from "react";
import Icon from "@/components/Icon";

export default function Field({ label, icon, hint, error, below, className = "", children, trailing }) {
  return (
    <label className={`field${error ? " has-error" : ""} ${className}`.trim()}>
      {label && <span className="field-label">{label}</span>}
      <span className={`field-box${icon ? " has-icon" : ""}${trailing ? " has-trailing" : ""}`}>
        {icon && <Icon name={icon} size={17} className="field-icon" />}
        {children}
        {trailing}
      </span>
      {below}
      {error && (
        <span className="form-error field-error" role="alert">
          {error}
        </span>
      )}
      {hint && <span className="hint">{hint}</span>}
    </label>
  );
}

export function PasswordField({ label = "Password", hint, error, below, ...input }) {
  const [shown, setShown] = useState(false);
  return (
    <Field
      label={label}
      icon="lock"
      hint={hint}
      error={error}
      below={below}
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
