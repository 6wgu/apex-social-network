"use client";

import { useState } from "react";
import { api } from "@/lib/api";

export default function GroupJoinButton({ group, onChange }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function run(path, method) {
    setBusy(true);
    setError("");
    try {
      await api(path, { method });
    } catch (err) {
      if (err.status !== 409 && err.status !== 404) setError(err.message);
    }
    await onChange();
    setBusy(false);
  }

  const request = `/api/groups/${group.id}/request`;
  let buttons;
  if (group.my_status === "none") {
    buttons = (
      <button className="btn-primary" disabled={busy} onClick={() => run(request, "POST")}>
        Request to join
      </button>
    );
  } else if (group.my_status === "requested") {
    buttons = (
      <button
        className="follow-btn follow-pending btn-secondary"
        disabled={busy}
        onClick={() => run(request, "DELETE")}
        aria-label="Cancel request"
      >
        <span className="when-idle">Requested</span>
        <span className="when-hover">Cancel request</span>
      </button>
    );
  } else if (group.my_status === "invited" && group.invite_id) {
    const invite = `/api/group-invites/${group.invite_id}`;
    buttons = (
      <>
        <button className="btn-primary" disabled={busy} onClick={() => run(`${invite}/accept`, "POST")}>
          Accept invitation
        </button>
        <button className="btn-secondary" disabled={busy} onClick={() => run(`${invite}/decline`, "POST")}>
          Decline
        </button>
      </>
    );
  } else {
    return null;
  }

  return (
    <div className="group-join">
      <div className="group-join-buttons">{buttons}</div>
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}
