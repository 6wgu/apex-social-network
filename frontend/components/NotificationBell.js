"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { timeAgo } from "@/lib/time";
import Avatar from "@/components/Avatar";
import { useRealtime, useRealtimeConnected } from "@/components/RealtimeProvider";

// Tells open pages (e.g. a profile showing follower counts) that a follow
// changed because of something done in the bell.
export const FOLLOWS_CHANGED = "follows-changed";

function describe(n) {
  const name = n.actor ? `${n.actor.first_name} ${n.actor.last_name}` : "Someone";
  switch (n.type) {
    case "follow_request":
      return { name, text: "wants to follow you" };
    case "new_follower":
      return { name, text: "started following you" };
    case "follow_accepted":
      return { name, text: "accepted your follow request" };
    default:
      return { name, text: "sent you a notification" };
  }
}

// The bell in the navbar: unread badge, dropdown list, Accept / Decline on
// follow requests. It loads from the API and then stays current through
// the WebSocket (notification / notification_removed events). After a
// reconnect it reloads, because events sent while offline are not replayed.
export default function NotificationBell() {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState(null);
  const [unread, setUnread] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(null); // id of the notification being answered
  const connected = useRealtimeConnected();
  const wrapper = useRef(null);

  const refreshCount = useCallback(() => {
    api("/api/notifications/unread-count")
      .then((c) => setUnread(c.unread))
      .catch(() => {});
  }, []);

  const load = useCallback(() => {
    refreshCount();
    api("/api/notifications")
      .then((list) => {
        setItems(list);
        setError("");
      })
      .catch((err) => setError(err.message));
  }, [refreshCount]);

  useEffect(load, [load]);
  useEffect(() => {
    if (connected) load();
  }, [connected, load]);

  useRealtime((event) => {
    if (event.type === "notification") {
      setItems((list) => (list ? [event.payload, ...list.filter((n) => n.id !== event.payload.id)] : list));
      refreshCount();
    } else if (event.type === "notification_removed") {
      setItems((list) => list && list.filter((n) => n.id !== event.payload.id));
      refreshCount();
    }
  });

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

  function toggle() {
    if (!open) load();
    setOpen(!open);
  }

  async function markRead(n) {
    if (n.is_read) return;
    setItems((list) => list.map((x) => (x.id === n.id ? { ...x, is_read: true } : x)));
    setUnread((u) => Math.max(0, u - 1));
    api(`/api/notifications/${n.id}/read`, { method: "POST" }).catch(refreshCount);
  }

  async function markAllRead() {
    setItems((list) => list.map((x) => ({ ...x, is_read: true })));
    setUnread(0);
    api("/api/notifications/read-all", { method: "POST" }).catch(refreshCount);
  }

  async function answer(n, accept) {
    setBusy(n.id);
    setError("");
    try {
      await api(`/api/follow-requests/${n.actor.id}/${accept ? "accept" : "decline"}`, { method: "POST" });
    } catch (err) {
      // 404: already answered (another tab, or the requester cancelled).
      if (err.status !== 404) {
        setError(err.message);
        setBusy(null);
        return;
      }
    }
    // The server deletes the notification and pushes notification_removed;
    // drop it here too so the list updates even without the WebSocket.
    setItems((list) => list.filter((x) => x.id !== n.id));
    refreshCount();
    setBusy(null);
    window.dispatchEvent(new CustomEvent(FOLLOWS_CHANGED));
  }

  return (
    <div className="bell" ref={wrapper}>
      <button
        className="icon-btn"
        aria-label={unread ? `Notifications, ${unread} unread` : "Notifications"}
        aria-expanded={open}
        onClick={toggle}
      >
        🔔
        <span className="badge badge-notification" hidden={!unread}>
          {unread > 99 ? "99+" : unread}
        </span>
      </button>

      {open && (
        <div className="bell-panel" role="dialog" aria-label="Notifications">
          <div className="bell-head">
            <strong>Notifications</strong>
            {unread > 0 && (
              <button className="link-btn" onClick={markAllRead}>
                Mark all as read
              </button>
            )}
          </div>
          {error && <p className="form-error bell-error">{error}</p>}
          {!items && !error && <p className="muted bell-empty">Loading…</p>}
          {items && items.length === 0 && <p className="muted bell-empty">No notifications yet.</p>}
          <ul className="bell-list">
            {(items || []).map((n) => {
              const { name, text } = describe(n);
              return (
                <li key={n.id} className={n.is_read ? "" : "unread"}>
                  <Link
                    href={n.actor ? `/profile/${n.actor.id}` : "#"}
                    className="bell-item"
                    onClick={() => {
                      markRead(n);
                      setOpen(false);
                    }}
                  >
                    <Avatar user={n.actor} size={40} />
                    <span className="bell-text">
                      <span>
                        <strong>{name}</strong> {text}
                      </span>
                      <span className="muted bell-time">{timeAgo(n.created_at)}</span>
                    </span>
                    {!n.is_read && <span className="unread-dot" aria-label="unread" />}
                  </Link>
                  {n.type === "follow_request" && n.actor && (
                    <div className="bell-actions">
                      <button className="btn-primary" disabled={busy === n.id} onClick={() => answer(n, true)}>
                        Accept
                      </button>
                      <button className="btn-secondary" disabled={busy === n.id} onClick={() => answer(n, false)}>
                        Decline
                      </button>
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </div>
  );
}
