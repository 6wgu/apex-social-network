"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import { echoQuery } from "@/lib/search";
import { timeAgo } from "@/lib/time";
import useAutoRefresh from "@/lib/useAutoRefresh";
import useSearchQuery from "@/lib/useSearchQuery";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import ChatWindow from "@/components/ChatWindow";
import Icon from "@/components/Icon";
import Modal from "@/components/Modal";
import RacingLights, { Loading } from "@/components/RacingLights";
import { useRealtime, useRealtimeConnected, useRealtimeSend } from "@/components/RealtimeProvider";
import SearchInput from "@/components/SearchInput";

const NOT_ALLOWED = "You can only message people you follow or who follow you.";
const TYPING_SHOWN = 4000; // ms: as in ChatWindow, "typing…" goes away when no new typing event came for this long

function name(u) {
  return `${u.first_name} ${u.last_name}`;
}

// /chats: my private conversations on the left, the open one on the right
// (?user=ID opens a chat, e.g. from a profile's Message button). On a phone
// it shows the list or the chat, with a back button.
// "New chat": everyone I may message (I follow them or they follow me),
// with a search, so a chat can start here without visiting a profile.
function NewChatDialog({ people, onPick, onClose }) {
  const [q, setQ] = useState("");
  const needle = q.trim().toLowerCase();
  const shown = people.filter(
    (u) => !needle || name(u).toLowerCase().includes(needle) || (u.nickname || "").toLowerCase().includes(needle)
  );

  return (
    <Modal title="New chat" onClose={onClose}>
      {people.length === 0 ? (
        <p className="muted">
          You can message people you follow or who follow you. <Link href="/search">Find people</Link> to follow first.
        </p>
      ) : (
        <>
          <label className="chat-filter">
            <Icon name="search" size={16} />
            <SearchInput value={q} onChange={setQ} placeholder="Search people you follow" aria-label="Search people" />
          </label>
          <ul className="chat-conversations new-chat-list">
            {shown.map((u) => (
              <li key={u.id}>
                <button className="chat-item" onClick={() => onPick(u.id)}>
                  <Avatar user={u} size={40} />
                  <span className="chat-item-text">
                    <strong>{name(u)}</strong>
                    {u.nickname && <span className="chat-item-last">@{u.nickname}</span>}
                  </span>
                </button>
              </li>
            ))}
          </ul>
          {shown.length === 0 && <p className="muted chat-filter-empty">Nobody matches “{echoQuery(q.trim())}”.</p>}
        </>
      )}
    </Modal>
  );
}

function Chats() {
  const { user: me } = useAuth();
  const router = useRouter();
  const params = useSearchParams();
  const selectedId = Number(params.get("user")) || null;
  const send = useRealtimeSend();
  const connected = useRealtimeConnected();

  const [conversations, setConversations] = useState(null);
  const [contacts, setContacts] = useState([]);
  const [error, setError] = useState("");
  const [stranger, setStranger] = useState(null); // ?user= someone with no conversation and not a contact
  const [filter, setFilter] = useState(""); // narrows the list by name, in the browser only
  const [picking, setPicking] = useState(false); // the "New chat" dialog is open
  const filterQuery = useSearchQuery(filter);
  const query = filterQuery.toLowerCase();

  const load = useCallback(() => {
    Promise.all([api("/api/conversations"), api("/api/chat-contacts")])
      .then(([convs, people]) => {
        setConversations(convs);
        setContacts(people);
        setError("");
      })
      .catch((err) => setError(err.message));
  }, []);

  useEffect(load, [load]);
  useAutoRefresh(load, 30000);
  // Reconnected: the list may have missed messages.
  useEffect(() => {
    if (connected) load();
  }, [connected, load]);

  const markRead = useCallback((otherId) => {
    setConversations((list) => list && list.map((c) => (c.user.id === otherId ? { ...c, unread_count: 0 } : c)));
    api(`/api/messages/${otherId}/read`, { method: "POST" }).catch(() => {});
  }, []);

  // Who is typing to me right now (user id -> timer), so the list shows
  // "typing…" even when that chat is not the one open.
  const [typing, setTyping] = useState(() => new Set());
  const typingTimers = useRef(new Map());
  const stopTyping = useCallback((userId) => {
    clearTimeout(typingTimers.current.get(userId));
    typingTimers.current.delete(userId);
    setTyping((old) => {
      if (!old.has(userId)) return old;
      const next = new Set(old);
      next.delete(userId);
      return next;
    });
  }, []);
  useEffect(() => {
    const timers = typingTimers.current;
    return () => timers.forEach(clearTimeout);
  }, []);

  useRealtime((event) => {
    if (event.type === "typing") {
      const p = event.payload;
      // Only private chats with me; my own tab's echo has a request_id.
      if (event.request_id || !p?.user || p.recipient_id !== me.id) return;
      const who = p.user.id;
      setTyping((old) => (old.has(who) ? old : new Set(old).add(who)));
      clearTimeout(typingTimers.current.get(who));
      typingTimers.current.set(who, setTimeout(() => stopTyping(who), TYPING_SHOWN));
      return;
    }
    if (event.type === "messages_read") {
      setConversations((list) => list && list.map((c) => (c.user.id === event.payload.user_id ? { ...c, unread_count: 0 } : c)));
      return;
    }
    if (event.type !== "private_message") return;
    const m = event.payload;
    const otherId = m.sender.id === me.id ? m.recipient_id : m.sender.id;
    if (m.sender.id !== me.id) stopTyping(m.sender.id); // their message is here: no longer typing
    setConversations((list) => {
      if (!list) return list;
      const existing = list.find((c) => c.user.id === otherId);
      if (!existing) {
        load(); // a new conversation: fetch it with the other user's details
        return list;
      }
      const fromThem = m.sender.id !== me.id;
      const updated = {
        ...existing,
        last_message: m,
        unread_count: fromThem && otherId !== selectedId ? existing.unread_count + 1 : existing.unread_count,
      };
      return [updated, ...list.filter((c) => c.user.id !== otherId)];
    });
  });

  const selectedConv = conversations?.find((c) => c.user.id === selectedId);
  const selectedContact = contacts.find((u) => u.id === selectedId);
  const partner = selectedConv?.user || selectedContact || stranger;
  const canMessage = selectedConv ? selectedConv.can_message : Boolean(selectedContact);
  const newContacts = useMemo(
    () => contacts.filter((u) => !conversations?.some((c) => c.user.id === u.id)),
    [contacts, conversations]
  );

  // ?user= someone who is neither a conversation nor a contact: load their
  // name so the page can say why chatting is not possible.
  useEffect(() => {
    setStranger(null);
    if (!selectedId || !conversations || selectedConv || selectedContact) return;
    api(`/api/users/${selectedId}`)
      .then((p) => setStranger(p.user))
      .catch(() => setStranger({ id: selectedId, first_name: "Unknown", last_name: "user", missing: true }));
  }, [selectedId, conversations, selectedConv, selectedContact]);

  const open = (id) => router.push(`/chats?user=${id}`);
  const matchesFilter = (u) => !query || name(u).toLowerCase().includes(query) || (u.nickname || "").toLowerCase().includes(query);
  const shownConversations = (conversations || []).filter((c) => matchesFilter(c.user));
  const shownContacts = newContacts.filter(matchesFilter);

  return (
    <div className={`chat-layout${selectedId ? " has-selection" : ""}`}>
      {picking && (
        <NewChatDialog
          people={contacts}
          onClose={() => setPicking(false)}
          onPick={(id) => {
            setPicking(false);
            open(id);
          }}
        />
      )}
      <aside className="card chat-list">
        <div className="chat-list-head">
          <div className="chat-list-title">
            <h1>Messages</h1>
            <button type="button" className="chat-header-action chat-new" onClick={() => setPicking(true)} aria-label="New chat" title="New chat">
              <Icon name="edit" size={17} />
            </button>
          </div>
          <label className="chat-filter">
            <Icon name="search" size={16} />
            <SearchInput value={filter} onChange={setFilter} placeholder="Search conversations" aria-label="Search conversations" />
          </label>
        </div>
        {error && <p className="form-error">{error}</p>}
        {!conversations && !error && <Loading className="muted" />}
        {conversations && conversations.length === 0 && newContacts.length === 0 && (
          <p className="muted">
            No chats yet. You can message people you follow or who follow you: find someone on the <Link href="/">home page</Link>.
          </p>
        )}
        <ul className="chat-conversations">
          {shownConversations.map((c) => (
            <li key={c.user.id}>
              <button className={`chat-item${c.user.id === selectedId ? " active" : ""}`} onClick={() => open(c.user.id)}>
                <Avatar user={c.user} size={44} />
                <span className="chat-item-text">
                  <span className="chat-item-top">
                    <strong>{name(c.user)}</strong>
                    <span className="muted chat-item-time">{timeAgo(c.last_message.created_at)}</span>
                  </span>
                  {typing.has(c.user.id) ? (
                    <span className="chat-item-last chat-item-typing">
                      <RacingLights mode="cycle" small />
                      typing…
                    </span>
                  ) : (
                    <span className={`chat-item-last${c.unread_count ? " unread" : ""}`}>
                      {c.last_message.sender.id === me.id ? "You: " : ""}
                      {c.last_message.content}
                    </span>
                  )}
                </span>
                {c.unread_count > 0 && (
                  <span className="chat-unread" aria-label={`${c.unread_count} unread`}>
                    {c.unread_count}
                  </span>
                )}
              </button>
            </li>
          ))}
        </ul>
        {query && conversations && shownConversations.length === 0 && shownContacts.length === 0 && (
          <p className="muted chat-filter-empty">Nobody matches “{echoQuery(filterQuery)}”.</p>
        )}
        {shownContacts.length > 0 && (
          <>
            <h2 className="chat-section">Start a new chat</h2>
            <ul className="chat-conversations">
              {shownContacts.map((u) => (
                <li key={u.id}>
                  <button className={`chat-item${u.id === selectedId ? " active" : ""}`} onClick={() => open(u.id)}>
                    <Avatar user={u} size={36} />
                    <span className="chat-item-text">
                      <strong>{name(u)}</strong>
                      {typing.has(u.id) && (
                        <span className="chat-item-last chat-item-typing">
                          <RacingLights mode="cycle" small />
                          typing…
                        </span>
                      )}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </>
        )}
      </aside>

      <section className="card chat-main">
        {!selectedId && (
          <div className="chat-placeholder">
            <span className="chat-placeholder-icon">
              <Icon name="chat" size={30} />
            </span>
            <h2>Your messages</h2>
            <p className="muted">Choose a conversation, or start a new one with someone you follow.</p>
          </div>
        )}
        {selectedId && partner && (
          <ChatWindow
            key={selectedId}
            historyUrl={`/api/messages/${selectedId}`}
            header={
              <header className="chat-header">
                <button className="chat-back" onClick={() => router.push("/chats")} aria-label="Back to all chats">
                  <Icon name="arrowLeft" size={18} />
                </button>
                <Link href={`/profile/${partner.id}`} className="chat-header-user">
                  <Avatar user={partner} size={42} />
                  <span className="chat-header-text">
                    <strong>{name(partner)}</strong>
                    <span>{partner.nickname ? `@${partner.nickname} · ` : ""}View profile</span>
                  </span>
                </Link>
                <Link href={`/profile/${partner.id}`} className="chat-header-action" aria-label={`${name(partner)}'s profile`}>
                  <Icon name="user" size={18} />
                </Link>
              </header>
            }
            matches={(e) =>
              e.type === "private_message" &&
              ((e.payload.sender.id === selectedId && e.payload.recipient_id === me.id) ||
                (e.payload.sender.id === me.id && e.payload.recipient_id === selectedId))
            }
            send={(content) => send("private_message", { recipient_id: selectedId, content })}
            canSend={canMessage && !partner.missing}
            cannotSend={partner.missing ? "This user does not exist." : `${NOT_ALLOWED} You can still read your earlier messages with ${partner.first_name}.`}
            onSeen={() => markRead(selectedId)}
          />
        )}
        {selectedId && !partner && conversations && <Loading className="muted center chat-placeholder" />}
      </section>
    </div>
  );
}

export default function ChatsPage() {
  return (
    <Suspense fallback={<Loading />}>
      <Chats />
    </Suspense>
  );
}
