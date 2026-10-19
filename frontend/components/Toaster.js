"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import Icon from "@/components/Icon";

const EVENT = "apex-notify";
const LIFETIME = 5000; // ms a toast stays before it closes on its own
const MAX = 4; // toasts shown at once; the oldest goes first

let nextId = 0;

/**
 * notify("Saved") or notify({ title, message, href, tone, duration })
 * Shows a toast from anywhere in the app (no hook or context needed).
 *   title     bold first line (optional)
 *   message   the text
 *   href      makes the toast a link (e.g. to the post or profile)
 *   tone      "info" (default), "success" or "error"
 *   duration  ms before it closes on its own; 0 keeps it until closed
 * Returns the toast's id.
 */
export function notify(input) {
  const toast = typeof input === "string" ? { message: input } : { ...input };
  toast.id = ++nextId;
  if (typeof window !== "undefined") window.dispatchEvent(new CustomEvent(EVENT, { detail: toast }));
  return toast.id;
}

const ICONS = { info: "bell", success: "check", error: "info" };

function Toast({ toast, onClose }) {
  const { title, message, href, tone = "info", duration = LIFETIME } = toast;
  const [leaving, setLeaving] = useState(false);
  const timer = useRef(null);

  const close = useCallback(() => {
    setLeaving(true);
    setTimeout(() => onClose(toast.id), 200);
  }, [onClose, toast.id]);

  const start = useCallback(() => {
    if (duration > 0) timer.current = setTimeout(close, duration);
  }, [close, duration]);

  useEffect(() => {
    start();
    return () => clearTimeout(timer.current);
  }, [start]);

  const body = (
    <>
      <span className="toast-icon">
        <Icon name={ICONS[tone] || "bell"} size={17} />
      </span>
      <span className="toast-text">
        {title && <strong>{title}</strong>}
        {message && <span>{message}</span>}
      </span>
    </>
  );

  return (
    // Hovering pauses the timer, so a toast never vanishes while being read.
    <li
      className={`toast toast-${tone}${leaving ? " is-leaving" : ""}`}
      onMouseEnter={() => clearTimeout(timer.current)}
      onMouseLeave={start}
    >
      {href ? (
        <Link href={href} className="toast-body" onClick={close}>
          {body}
        </Link>
      ) : (
        <div className="toast-body">{body}</div>
      )}
      <button type="button" className="toast-close" onClick={close} aria-label="Dismiss">
        <Icon name="x" size={14} />
      </button>
      {duration > 0 && <span className="toast-timer" style={{ animationDuration: `${duration}ms` }} />}
    </li>
  );
}

// Renders the toasts sent with notify(). Mounted once, in the root layout.
export default function Toaster() {
  const [toasts, setToasts] = useState([]);

  useEffect(() => {
    const add = (e) => setToasts((list) => [...list, e.detail].slice(-MAX));
    window.addEventListener(EVENT, add);
    return () => window.removeEventListener(EVENT, add);
  }, []);

  const remove = useCallback((id) => setToasts((list) => list.filter((t) => t.id !== id)), []);

  return (
    <ol className="toaster" aria-live="polite" aria-label="Notifications">
      {toasts.map((t) => (
        <Toast key={t.id} toast={t} onClose={remove} />
      ))}
    </ol>
  );
}
