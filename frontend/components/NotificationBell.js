"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { formatEventTime, timeAgo } from "@/lib/time";
import useAutoRefresh from "@/lib/useAutoRefresh";
import Avatar from "@/components/Avatar";
import Icon from "@/components/Icon";
import { notify } from "@/components/Toaster";
import { Loading } from "@/components/RacingLights";
import { useRealtime, useRealtimeConnected } from "@/components/RealtimeProvider";

// Tell open pages (e.g. a profile showing follower counts, or a group's
// member list) that something changed because of an answer given in the bell.
export const FOLLOWS_CHANGED = "follows-changed";
export const GROUPS_CHANGED = "groups-changed";

// The types that are answered with Accept / Decline right in the bell.
const ANSWERABLE = ["follow_request", "group_invite", "group_join_request"];

// What a notification says and where clicking it goes. target (bold, after
// the text) is the group's title; detail is an extra line.
function describe(n) {
  const name = n.actor ? `${n.actor.first_name} ${n.actor.last_name}` : "Someone";
  const profile = n.actor ? `/profile/${n.actor.id}` : "#";
  const group = n.group ? `/groups/${n.group.id}` : "/groups";
  const target = n.group?.title;
  switch (n.type) {
    case "follow_request":
      return { name, text: "wants to follow you", href: profile };
    case "new_follower":
      return { name, text: "started following you", href: profile };
    case "follow_accepted":
      return { name, text: "accepted your follow request", href: profile };
    case "group_invite":
      return { name, text: "invited you to join", target, href: group };
    case "group_join_request":
      return { name, text: "wants to join your group", target, href: `${group}?tab=Requests` };
    case "group_request_accepted":
      return { name, text: "accepted your request to join", target, href: group };
    case "group_event":
      return {
        name,
        text: "created an event in",
        target,
        detail: n.event && `📅 ${n.event.title} · ${formatEventTime(n.event.event_time)}`,
        href: `${group}?tab=Events`,
      };
    default:
      return { name, text: "sent you a notification", href: profile };
  }
}

// Sends Accept / Decline for an answerable notification. The notification
// only names the user and the group, so for the group types the id to
// answer is looked up first. A 404 means it was already answered.
async function sendAnswer(n, accept) {
  const verb = accept ? "accept" : "decline";
  if (n.type === "follow_request") {
    await api(`/api/follow-requests/${n.actor.id}/${verb}`, { method: "POST" });
  } else if (n.type === "group_invite") {
    const group = await api(`/api/groups/${n.group.id}`);
    if (!group.invite_id) throw new ApiError(404, "invitation not found");
    await api(`/api/group-invites/${group.invite_id}/${verb}`, { method: "POST" });
  } else if (n.type === "group_join_request") {
    const requests = await api(`/api/groups/${n.group.id}/requests`);
    const request = requests.find((r) => r.user.id === n.actor.id);
    if (!request) throw new ApiError(404, "join request not found");
    await api(`/api/group-requests/${request.id}/${verb}`, { method: "POST" });
  }
}

// The bell in the navbar: unread badge, dropdown list, Accept / Decline on
// follow requests, group invitations and join requests. It loads from the API and then stays current through
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
  const countRequest = useRef(0);

  const refreshCount = useCallback(() => {
    const request = ++countRequest.current;
    api("/api/notifications/unread-count")
      .then((c) => {
        if (request === countRequest.current) setUnread(c.unread);
      })
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
  // Stay current without a reload, even if a live event was missed.
  useAutoRefresh(load);
  useEffect(() => {
    if (connected) load();
  }, [connected, load]);

  useRealtime((event) => {
    if (event.type === "notification") {
      setItems((list) => (list ? [event.payload, ...list.filter((n) => n.id !== event.payload.id)] : list));
      refreshCount();
      const { name, text, target, href } = describe(event.payload);
      notify({ title: name, message: target ? `${text} ${target}` : text, href });
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
      await sendAnswer(n, accept);
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
    window.dispatchEvent(new CustomEvent(n.type === "follow_request" ? FOLLOWS_CHANGED : GROUPS_CHANGED));
  }

  return (
    <div className="bell" ref={wrapper}>
      <button
        type="button"
        className={`nav-item bell-trigger${open ? " active" : ""}`}
        aria-label={unread ? `Notifications, ${unread} unread` : "Notifications"}
        aria-expanded={open}
        onClick={toggle}
        title="Notifications"
      >
        <Icon name="bell" className="nav-icon" />
        <span className="nav-label">Notifications</span>
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
          {!items && !error && <Loading className="muted bell-empty" />}
          {items && items.length === 0 && <p className="muted bell-empty">No notifications yet.</p>}
          <ul className="bell-list">
            {(items || []).map((n) => {
              const { name, text, target, detail, href } = describe(n);
              return (
                <li key={n.id} className={n.is_read ? "" : "unread"}>
                  <Link
                    href={href}
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
                        {target && (
                          <>
                            {" "}
                            <strong>{target}</strong>
                          </>
                        )}
                      </span>
                      {detail && <span className="bell-detail">{detail}</span>}
                      <span className="muted bell-time">{timeAgo(n.created_at)}</span>
                    </span>
                    {!n.is_read && <span className="unread-dot" aria-label="unread" />}
                  </Link>
                  {ANSWERABLE.includes(n.type) && n.actor && (
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
