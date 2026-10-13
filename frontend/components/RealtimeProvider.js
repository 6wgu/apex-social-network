"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { useAuth } from "@/components/AuthProvider";

const RealtimeContext = createContext({ connected: false, subscribe: () => () => {} });

// Opens one WebSocket to /ws per tab while a user is logged in, and
// reconnects with a growing delay (1 s, 2 s, 4 s … up to 30 s) when it drops.
// The Next.js server proxies /ws to the backend, so the session cookie and
// the page's Origin go with the upgrade request.
//
// Components listen with useRealtime(handler); every event is
// { type, payload } as described in docs/api.md.
export default function RealtimeProvider({ children }) {
  const { user } = useAuth();
  const userId = user?.id;
  const listeners = useRef(new Set());
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (!userId) return;
    let socket;
    let retryTimer;
    let attempt = 0;
    let stopped = false;

    function connect() {
      const scheme = window.location.protocol === "https:" ? "wss" : "ws";
      socket = new WebSocket(`${scheme}://${window.location.host}/ws`);
      socket.onopen = () => {
        attempt = 0;
        setConnected(true);
      };
      socket.onmessage = (msg) => {
        let event;
        try {
          event = JSON.parse(msg.data);
        } catch {
          return;
        }
        listeners.current.forEach((fn) => fn(event));
      };
      socket.onclose = () => {
        setConnected(false);
        if (stopped) return;
        retryTimer = setTimeout(connect, Math.min(30000, 1000 * 2 ** attempt++));
      };
    }

    connect();
    return () => {
      stopped = true;
      clearTimeout(retryTimer);
      socket?.close();
      setConnected(false);
    };
  }, [userId]);

  const subscribe = useCallback((fn) => {
    listeners.current.add(fn);
    return () => listeners.current.delete(fn);
  }, []);

  return <RealtimeContext.Provider value={{ connected, subscribe }}>{children}</RealtimeContext.Provider>;
}

/** True while the WebSocket is open. Use it to reload data after a reconnect. */
export function useRealtimeConnected() {
  return useContext(RealtimeContext).connected;
}

/** Calls handler(event) for every WebSocket event while the component is mounted. */
export function useRealtime(handler) {
  const { subscribe } = useContext(RealtimeContext);
  const latest = useRef(handler);
  latest.current = handler;
  useEffect(() => subscribe((event) => latest.current(event)), [subscribe]);
}
