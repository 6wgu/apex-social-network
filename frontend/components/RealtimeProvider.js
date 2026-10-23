"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";

const RealtimeContext = createContext({
  connected: false,
  subscribe: () => () => {},
  send: () => Promise.reject(new Error("not connected")),
});

const SEND_TIMEOUT = 10000;

export default function RealtimeProvider({ children }) {
  const { user, logout } = useAuth();
  const userId = user?.is_verified ? user.id : null;
  const logoutRef = useRef(logout);
  logoutRef.current = logout;
  const listeners = useRef(new Set());
  const socketRef = useRef(null);
  const pending = useRef(new Map());
  const nextId = useRef(0);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (!userId) return;
    let socket;
    let retryTimer;
    let attempt = 0;
    let stopped = false;

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
        if (stopped) return;
        setConnected(false);
        failPending("connection lost, please try again");
        api("/api/me", { redirectOn401: false })
          .then(() => {
            if (!stopped) retryTimer = setTimeout(connect, Math.min(30000, 1000 * 2 ** attempt++));
          })
          .catch((err) => {
            if (stopped) return;
            if (err.status === 401) {
              stopped = true;
              logoutRef.current({ quiet: true });
              return;
            }
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

export function useRealtimeConnected() {
  return useContext(RealtimeContext).connected;
}

export function useRealtimeSend() {
  return useContext(RealtimeContext).send;
}

export function useRealtime(handler) {
  const { subscribe } = useContext(RealtimeContext);
  const latest = useRef(handler);
  latest.current = handler;
  useEffect(() => subscribe((event) => latest.current(event)), [subscribe]);
}
