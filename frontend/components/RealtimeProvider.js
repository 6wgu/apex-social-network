"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";

const RealtimeContext = createContext({
  connected: false,
  subscribe: () => () => {},
  send: () => Promise.reject(new Error("not connected")),
});

const SEND_TIMEOUT = 10000; // ms to wait for the server's answer to a frame

// Opens one WebSocket to /ws per tab while a user is logged in, and
// reconnects with a growing delay (1 s, 2 s, 4 s … up to 30 s) when it drops,
// unless the session is gone, in which case it logs out.
// The Next.js server proxies /ws to the backend, so the session cookie and
// the page's Origin go with the upgrade request.
//
// Components listen with useRealtime(handler); every event is
// { type, payload } as described in docs/api.md. useRealtimeSend() returns
// send(type, payload), which resolves with the server's answer to that
// frame (matched by request_id) or rejects with its error.
export default function RealtimeProvider({ children }) {
  const { user, logout } = useAuth();
  const userId = user?.id;
  const logoutRef = useRef(logout);
  logoutRef.current = logout;
  const listeners = useRef(new Set());
  const socketRef = useRef(null);
  const pending = useRef(new Map()); // request_id -> { resolve, reject, timer }
  const nextId = useRef(0);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (!userId) return;
    let socket;
    let retryTimer;
    let attempt = 0;
    let stopped = false;

    // Every frame still waiting for an answer fails when the socket goes.
    function failPending(reason) {
      pending.current.forEach(({ reject, timer }) => {
        clearTimeout(timer);
        reject(new Error(reason));
      });
      pending.current.clear();
    }

    function connect() {
      const scheme = window.location.protocol === "https:" ? "wss" : "ws";
      socket = new WebSocket(`${scheme}://${window.location.host}/ws`);
      socketRef.current = socket;
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
        // An answer to a frame this tab sent: settle its promise.
        const waiting = event.request_id && pending.current.get(event.request_id);
        if (waiting) {
          pending.current.delete(event.request_id);
          clearTimeout(waiting.timer);
          if (event.type === "error") waiting.reject(new Error(event.payload?.message || "rejected"));
          else waiting.resolve(event.payload);
        }
        listeners.current.forEach((fn) => fn(event));
      };
      socket.onclose = () => {
        setConnected(false);
        failPending("connection lost, please try again");
        if (stopped) return;
        // The socket also closes when the session ends (logout in another
        // tab, or expiry): ask the API before retrying. A 401 means there is
        // nothing to reconnect to, so log out instead of retrying forever.
        api("/api/me", { redirectOn401: false })
          .then(() => {
            if (!stopped) retryTimer = setTimeout(connect, Math.min(30000, 1000 * 2 ** attempt++));
          })
          .catch((err) => {
            if (stopped) return;
            if (err.status === 401) {
              stopped = true;
              logoutRef.current();
              return;
            }
            // Backend unreachable: keep retrying with the growing delay.
            retryTimer = setTimeout(connect, Math.min(30000, 1000 * 2 ** attempt++));
          });
      };
    }

    connect();
    return () => {
      stopped = true;
      clearTimeout(retryTimer);
      socket?.close();
      socketRef.current = null;
      failPending("logged out");
      setConnected(false);
    };
  }, [userId]);

  const send = useCallback((type, payload) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return Promise.reject(new Error("not connected, retrying…"));
    }
    const requestId = `r${Date.now().toString(36)}-${nextId.current++}`;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pending.current.delete(requestId);
        reject(new Error("no answer from the server"));
      }, SEND_TIMEOUT);
      pending.current.set(requestId, { resolve, reject, timer });
      socket.send(JSON.stringify({ type, request_id: requestId, payload }));
    });
  }, []);

  const subscribe = useCallback((fn) => {
    listeners.current.add(fn);
    return () => listeners.current.delete(fn);
  }, []);

  return <RealtimeContext.Provider value={{ connected, subscribe, send }}>{children}</RealtimeContext.Provider>;
}

/** True while the WebSocket is open. Use it to reload data after a reconnect. */
export function useRealtimeConnected() {
  return useContext(RealtimeContext).connected;
}

/** send(type, payload) -> Promise of the server's answer (see RealtimeProvider). */
export function useRealtimeSend() {
  return useContext(RealtimeContext).send;
}

/** Calls handler(event) for every WebSocket event while the component is mounted. */
export function useRealtime(handler) {
  const { subscribe } = useContext(RealtimeContext);
  const latest = useRef(handler);
  latest.current = handler;
  useEffect(() => subscribe((event) => latest.current(event)), [subscribe]);
}
