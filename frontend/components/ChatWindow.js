"use client";

import Link from "next/link";
import { Fragment, useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { parseTime } from "@/lib/time";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import EmojiPicker from "@/components/EmojiPicker";
import Icon from "@/components/Icon";
import { Loading } from "@/components/RacingLights";
import { useRealtime, useRealtimeConnected } from "@/components/RealtimeProvider";

const PAGE = 50; // the backend's page size for chat history
const MAX_LEN = 2000; // characters, as the backend counts them (code points)

function formatTime(s) {
  const d = parseTime(s);
  const sameDay = d.toDateString() === new Date().toDateString();
  return sameDay
    ? d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })
    : d.toLocaleString(undefined, { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
}

// "Today", "Yesterday", or the date, for the separators between days.
function dayLabel(d) {
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  if (d.toDateString() === today.toDateString()) return "Today";
  if (d.toDateString() === yesterday.toDateString()) return "Yesterday";
  return d.toLocaleDateString(undefined, { weekday: "short", day: "numeric", month: "short", year: d.getFullYear() === today.getFullYear() ? undefined : "numeric" });
}

// One chat: the history (oldest at the top, older pages loaded on scroll up),
// live messages, and the message box with an emoji picker. Used for private
// chats and for group chats; the caller says how:
//   historyUrl  REST history, newest first, paged with ?before=
//   matches     (event) => true for live events that belong to this chat
//   send        (content) => Promise of the stored message (over the WebSocket)
//   canSend     false shows cannotSend instead of the message box
//   showNames   show the sender's name on others' messages (group chat)
//   onSeen      called after messages from others are shown (to mark them read)
export default function ChatWindow({ historyUrl, header, matches, send, canSend = true, cannotSend, showNames = false, onSeen }) {
  const { user: me } = useAuth();
  const connected = useRealtimeConnected();
  const [messages, setMessages] = useState(null); // oldest first
  const [hasMore, setHasMore] = useState(false);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [error, setError] = useState("");
  const [text, setText] = useState("");
  const [sendError, setSendError] = useState("");
  const [sending, setSending] = useState(false);
  const list = useRef(null);
  const input = useRef(null);
  const keepScroll = useRef(null); // scrollHeight before older messages were added on top
  const stickToBottom = useRef(true);
  const wasConnected = useRef(connected);

  const onSeenRef = useRef(onSeen);
  onSeenRef.current = onSeen;

  // Adds messages, oldest first, without duplicates (a sent message comes
  // back both as the answer to send() and as a live event).
  const merge = useCallback((incoming) => {
    setMessages((old) => {
      const byId = new Map((old || []).map((m) => [m.id, m]));
      incoming.forEach((m) => byId.set(m.id, m));
      return [...byId.values()].sort((a, b) => a.id - b.id);
    });
  }, []);

  // First page.
  useEffect(() => {
    let current = true;
    setMessages(null);
    setError("");
    setHasMore(false);
    stickToBottom.current = true;
    api(historyUrl)
      .then((page) => {
        if (!current) return;
        setMessages([...page].reverse());
        setHasMore(page.length === PAGE);
        onSeenRef.current?.();
      })
      .catch((err) => current && setError(err.message));
    return () => {
      current = false;
    };
  }, [historyUrl]);

  // After a reconnect, fetch the newest page again: anything sent while
  // this tab was disconnected is merged in.
  useEffect(() => {
    if (connected && !wasConnected.current && messages) {
      api(historyUrl)
        .then((page) => {
          merge(page);
          onSeenRef.current?.();
        })
        .catch(() => {});
    }
    wasConnected.current = connected;
  }, [connected, historyUrl, merge, messages]);

  useRealtime((event) => {
    if (!matches(event)) return;
    const box = list.current;
    stickToBottom.current = !box || box.scrollHeight - box.scrollTop - box.clientHeight < 80 || event.payload.sender.id === me.id;
    merge([event.payload]);
    if (event.payload.sender.id !== me.id) onSeenRef.current?.();
  });

  async function loadOlder() {
    if (!messages?.length || loadingOlder || !hasMore) return;
    setLoadingOlder(true);
    try {
      const sep = historyUrl.includes("?") ? "&" : "?";
      const page = await api(`${historyUrl}${sep}before=${messages[0].id}`);
      keepScroll.current = list.current?.scrollHeight ?? null;
      stickToBottom.current = false;
      merge(page);
      setHasMore(page.length === PAGE);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoadingOlder(false);
    }
  }

  // Keep the view steady: at the bottom for new messages, at the same
  // message when older ones are added on top.
  useLayoutEffect(() => {
    const box = list.current;
    if (!box) return;
    if (keepScroll.current != null) {
      box.scrollTop += box.scrollHeight - keepScroll.current;
      keepScroll.current = null;
    } else if (stickToBottom.current) {
      box.scrollTop = box.scrollHeight;
    }
  }, [messages]);

  function onScroll(e) {
    if (e.currentTarget.scrollTop < 40) loadOlder();
  }

  function insertEmoji(emoji) {
    const el = input.current;
    const start = el?.selectionStart ?? text.length;
    const end = el?.selectionEnd ?? text.length;
    const next = text.slice(0, start) + emoji + text.slice(end);
    setText(next);
    requestAnimationFrame(() => {
      el?.focus();
      el?.setSelectionRange(start + emoji.length, start + emoji.length);
    });
  }

  async function submit(e) {
    e?.preventDefault();
    const content = text.trim();
    if (!content || sending) return;
    setSending(true);
    setSendError("");
    try {
      const stored = await send(content);
      stickToBottom.current = true;
      merge([stored]);
      setText("");
    } catch (err) {
      setSendError(err.message);
    } finally {
      setSending(false);
      input.current?.focus();
    }
  }

  function onKeyDown(e) {
    if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      submit();
    }
  }

  const length = [...text].length; // code points, like the backend

  return (
    <div className="chat-window">
      {header}
      <div className="chat-messages" ref={list} onScroll={onScroll} aria-live="polite">
        {hasMore && (
          <button className="link-btn chat-older" onClick={loadOlder} disabled={loadingOlder}>
            {loadingOlder ? "Loading…" : "Load older messages"}
          </button>
        )}
        {error && <p className="form-error">{error}</p>}
        {!messages && !error && <Loading className="muted center chat-empty">Loading messages…</Loading>}
        {messages && messages.length === 0 && <p className="muted center chat-empty">No messages yet. Say hello 👋</p>}
        {(messages || []).map((m, i) => {
          const mine = m.sender.id === me.id;
          const day = parseTime(m.created_at).toDateString();
          const newDay = i === 0 || parseTime(messages[i - 1].created_at).toDateString() !== day;
          const sameAsPrevious = !newDay && messages[i - 1].sender.id === m.sender.id;
          return (
            <Fragment key={m.id}>
            {newDay && (
              <div className="chat-day" role="separator">
                <span>{dayLabel(parseTime(m.created_at))}</span>
              </div>
            )}
            <div className={`chat-row ${mine ? "mine" : "theirs"}${sameAsPrevious ? " grouped" : ""}`}>
              {!mine && (
                <Link href={`/profile/${m.sender.id}`} className="chat-avatar" aria-hidden={sameAsPrevious} tabIndex={sameAsPrevious ? -1 : 0}>
                  {!sameAsPrevious && <Avatar user={m.sender} size={30} />}
                </Link>
              )}
              <div className="chat-bubble">
                {showNames && !mine && !sameAsPrevious && (
                  <Link href={`/profile/${m.sender.id}`} className="chat-name">
                    {m.sender.first_name} {m.sender.last_name}
                  </Link>
                )}
                <p className="chat-text">{m.content}</p>
                <time className="chat-time" dateTime={parseTime(m.created_at).toISOString()}>
                  {formatTime(m.created_at)}
                </time>
              </div>
            </div>
            </Fragment>
          );
        })}
      </div>
      {canSend ? (
        <form className="chat-input" onSubmit={submit}>
          <EmojiPicker onPick={insertEmoji} />
          <textarea
            ref={input}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder="Write a message…"
            title="Enter to send, Shift+Enter for a new line"
            rows={1}
            aria-label="Message"
          />
          <button className="btn-primary chat-send" disabled={sending || !text.trim() || length > MAX_LEN} aria-label="Send">
            <Icon name="send" size={18} />
          </button>
          {(sendError || length > MAX_LEN) && (
            <p className="form-error chat-send-error" role="alert">
              {length > MAX_LEN ? `Too long: ${length}/${MAX_LEN} characters` : sendError}
            </p>
          )}
        </form>
      ) : (
        <p className="chat-blocked muted">{cannotSend}</p>
      )}
    </div>
  );
}
