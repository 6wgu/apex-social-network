"use client";

import { createContext, useCallback, useContext, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";
import { useRealtime, useRealtimeConnected } from "@/components/RealtimeProvider";

const UnreadContext = createContext({ unread: 0, refresh: () => {} });

// The number of unread private messages, for the chat badge. It is kept
// apart from the notification count on purpose: new messages and new
// notifications look different. It is loaded from the API, goes up live when
// someone else's message arrives, and is loaded again after every
// (re)connect and whenever a conversation is marked read (messages_read), so
// nothing missed while offline is lost.
export default function UnreadMessagesProvider({ children }) {
  const { user } = useAuth();
  const connected = useRealtimeConnected();
  const [unread, setUnread] = useState(0);

  const refresh = useCallback(() => {
    if (!user) return;
    api("/api/messages/unread-count", { redirectOn401: false })
      .then((c) => setUnread(c.unread))
      .catch(() => {});
  }, [user]);

  useEffect(() => {
    if (!user) setUnread(0);
    else refresh();
  }, [user, refresh]);

  // A reconnect may have missed messages: count again.
  useEffect(() => {
    if (connected) refresh();
  }, [connected, refresh]);

  useRealtime((event) => {
    if (event.type === "private_message" && user && event.payload.sender?.id !== user.id) {
      setUnread((n) => n + 1);
    } else if (event.type === "messages_read") {
      refresh();
    }
  });

  return <UnreadContext.Provider value={{ unread, refresh }}>{children}</UnreadContext.Provider>;
}

/** { unread, refresh } for the chat badge and the chat page. */
export function useUnreadMessages() {
  return useContext(UnreadContext);
}
