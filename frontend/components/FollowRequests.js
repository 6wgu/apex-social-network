"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import Avatar from "@/components/Avatar";
import Icon from "@/components/Icon";
import { FOLLOWS_CHANGED } from "@/components/NotificationBell";
import { useRealtime } from "@/components/RealtimeProvider";

export default function FollowRequests() {
  const [requests, setRequests] = useState(null);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(null);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    api("/api/follow-requests")
      .then((list) => {
        setRequests(list);
        setError("");
      })
      .catch((err) => setError(err.message));
  }, []);

  useEffect(() => {
    load();
    window.addEventListener(FOLLOWS_CHANGED, load);
    return () => window.removeEventListener(FOLLOWS_CHANGED, load);
  }, [load]);

  useRealtime((event) => {
    if (event.type === "notification" || event.type === "notification_removed") load();
  });

  async function answer(userId, accept) {
    setBusy(userId);
    setError("");
    try {
      await api(`/api/follow-requests/${userId}/${accept ? "accept" : "decline"}`, { method: "POST" });
    } catch (err) {
      if (err.status !== 404) setError(err.message);
    }
    setBusy(null);
    setRequests((list) => list.filter((r) => r.user.id !== userId));
    window.dispatchEvent(new CustomEvent(FOLLOWS_CHANGED));
  }

  const count = requests?.length || 0;

  return (
    <section className="side-card follow-requests">
      <button type="button" className="side-toggle" onClick={() => setOpen((o) => !o)} aria-expanded={open}>
        <Icon name="userCheck" size={18} />
        <span>Follow requests</span>
        {count > 0 && <span className="count-badge">{count}</span>}
        <Icon name="chevronDown" size={16} className="side-toggle-chevron" />
      </button>
      {open && (
        <div className="side-toggle-body">
          {error && <p className="form-error">{error}</p>}
          {requests && count === 0 && <p className="side-empty">No pending requests.</p>}
          <ul className="side-list">
            {(requests || []).map((r) => (
              <li key={r.user.id} className="request-row">
                <Link href={`/profile/${r.user.id}`} className="side-row">
                  <Avatar user={r.user} size={34} />
                  <span className="side-row-name">
                    <span className="side-row-title">
                      {r.user.first_name} {r.user.last_name}
                    </span>
                  </span>
                </Link>
                <span className="request-actions">
                  <button className="btn-primary btn-small" disabled={busy === r.user.id} onClick={() => answer(r.user.id, true)}>
                    Accept
                  </button>
                  <button className="btn-secondary btn-small" disabled={busy === r.user.id} onClick={() => answer(r.user.id, false)}>
                    Decline
                  </button>
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
