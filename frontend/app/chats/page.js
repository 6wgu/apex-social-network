"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import { timeAgo } from "@/lib/time";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import ChatWindow from "@/components/ChatWindow";
import { useRealtime, useRealtimeConnected, useRealtimeSend } from "@/components/RealtimeProvider";

const NOT_ALLOWED = "You can only message people you follow or who follow you.";

function name(u) {
  return `${u.first_name} ${u.last_name}`;
}

// /chats: my private conversations on the left, the open one on the right
// (?user=ID opens a chat, e.g. from a profile's Message button). On a phone
// it shows the list or the chat, with a back button.
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
  // Reconnected: the list may have missed messages.
  useEffect(() => {
    if (connected) load();
  }, [connected, load]);

  const markRead = useCallback((otherId) => {
    setConversations((list) => list && list.map((c) => (c.user.id === otherId ? { ...c, unread_count: 0 } : c)));
    api(`/api/messages/${otherId}/read`, { method: "POST" }).catch(() => {});
  }, []);

  useRealtime((event) => {
    if (event.type === "messages_read") {
      setConversations((list) => list && list.map((c) => (c.user.id === event.payload.user_id ? { ...c, unread_count: 0 } : c)));
      return;
    }
    if (event.type !== "private_message") return;
    const m = event.payload;
    const otherId = m.sender.id === me.id ? m.recipient_id : m.sender.id;
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

  return (
    <div className={`chat-layout${selectedId ? " has-selection" : ""}`}>
      <aside className="card chat-list">
        <h1>Chats</h1>
        {error && <p className="form-error">{error}</p>}
        {!conversations && !error && <p className="muted">Loading…</p>}
        {conversations && conversations.length === 0 && newContacts.length === 0 && (
          <p className="muted">
            No chats yet. You can message people you follow or who follow you: find someone on the <Link href="/">home page</Link>.
          </p>
        )}
        <ul className="chat-conversations">
          {(conversations || []).map((c) => (
            <li key={c.user.id}>
              <button className={`chat-item${c.user.id === selectedId ? " active" : ""}`} onClick={() => open(c.user.id)}>
                <Avatar user={c.user} size={44} />
                <span className="chat-item-text">
                  <span className="chat-item-top">
                    <strong>{name(c.user)}</strong>
                    <span className="muted chat-item-time">{timeAgo(c.last_message.created_at)}</span>
                  </span>
                  <span className={`chat-item-last${c.unread_count ? " unread" : ""}`}>
                    {c.last_message.sender.id === me.id ? "You: " : ""}
                    {c.last_message.content}
                  </span>
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
        {newContacts.length > 0 && (
          <>
            <h2 className="chat-section">Start a new chat</h2>
            <ul className="chat-conversations">
              {newContacts.map((u) => (
                <li key={u.id}>
                  <button className={`chat-item${u.id === selectedId ? " active" : ""}`} onClick={() => open(u.id)}>
                    <Avatar user={u} size={36} />
                    <span className="chat-item-text">
                      <strong>{name(u)}</strong>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </>
        )}
      </aside>

      <section className="card chat-main">
        {!selectedId && <p className="muted center chat-placeholder">Choose a conversation, or start a new one.</p>}
        {selectedId && partner && (
          <ChatWindow
            key={selectedId}
            historyUrl={`/api/messages/${selectedId}`}
            header={
              <header className="chat-header">
                <button className="link-btn chat-back" onClick={() => router.push("/chats")} aria-label="Back to all chats">
                  ←
                </button>
                <Link href={`/profile/${partner.id}`} className="chat-header-user">
                  <Avatar user={partner} size={36} />
                  <strong>{name(partner)}</strong>
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
        {selectedId && !partner && conversations && <p className="muted center chat-placeholder">Loading…</p>}
      </section>
    </div>
  );
}

export default function ChatsPage() {
  return (
    <Suspense fallback={<p className="muted center">Loading…</p>}>
      <Chats />
    </Suspense>
  );
}
