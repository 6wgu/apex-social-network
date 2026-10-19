"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { timeAgo } from "@/lib/time";
import Avatar from "@/components/Avatar";
import { Loading } from "@/components/RacingLights";

// The creator's panel of pending join requests (GET /api/groups/{id}/requests)
// with Accept / Decline. Only the creator sees this tab, and the server
// answers 403 to anyone else anyway. onAnswered() lets the page reload the
// member list and count.
export default function JoinRequests({ groupId, reloadKey, onAnswered }) {
  const [requests, setRequests] = useState(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(null);

  const load = useCallback(
    () =>
      api(`/api/groups/${groupId}/requests`)
        .then((list) => {
          setRequests(list);
          setError("");
        })
        .catch((err) => setError(err.message)),
    [groupId]
  );

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  async function answer(request, accept) {
    setBusy(request.id);
    setError("");
    try {
      await api(`/api/group-requests/${request.id}/${accept ? "accept" : "decline"}`, { method: "POST" });
    } catch (err) {
      // 404: already answered, or cancelled by the requester.
      if (err.status !== 404) {
        setError(err.message);
        setBusy(null);
        return;
      }
    }
    setRequests((list) => list.filter((r) => r.id !== request.id));
    setBusy(null);
    onAnswered();
  }

  if (error) return <p className="form-error tab-placeholder">{error}</p>;
  if (!requests) return <Loading className="muted tab-placeholder" />;
  if (requests.length === 0) return <p className="muted tab-placeholder">No pending requests to join.</p>;

  return (
    <ul className="invite-list request-list">
      {requests.map((r) => (
        <li key={r.id}>
          <Link href={`/profile/${r.user.id}`}>
            <Avatar user={r.user} size={40} />
          </Link>
          <span className="invite-name">
            <Link href={`/profile/${r.user.id}`}>
              <strong>
                {r.user.first_name} {r.user.last_name}
              </strong>
            </Link>
            <span className="muted"> asked {timeAgo(r.created_at)}</span>
          </span>
          <span className="request-actions">
            <button className="btn-primary" disabled={busy === r.id} onClick={() => answer(r, true)}>
              Accept
            </button>
            <button className="btn-secondary" disabled={busy === r.id} onClick={() => answer(r, false)}>
              Decline
            </button>
          </span>
        </li>
      ))}
    </ul>
  );
}
