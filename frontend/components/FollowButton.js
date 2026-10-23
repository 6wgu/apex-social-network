"use client";

import { useState } from "react";
import { api } from "@/lib/api";

export default function FollowButton({ userId, status, onChange }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function click() {
    setBusy(true);
    setError("");
    try {
      await api(`/api/users/${userId}/follow`, { method: status === "none" ? "POST" : "DELETE" });
    } catch (err) {
      if (err.status !== 409 && err.status !== 404) setError(err.message);
    }
    await onChange();
    setBusy(false);
  }

  const labels = {
    none: ["Follow", "Follow"],
    pending: ["Requested", "Cancel request"],
    following: ["Following", "Unfollow"],
  }[status];
  if (!labels) return null;

  return (
    <div className="follow">
      <button
        className={`follow-btn follow-${status} ${status === "none" ? "btn-primary" : "btn-secondary"}`}
        onClick={click}
        disabled={busy}
        aria-label={labels[1]}
      >
        <span className="when-idle">{labels[0]}</span>
        <span className="when-hover">{labels[1]}</span>
      </button>
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}
