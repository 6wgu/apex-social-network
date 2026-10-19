"use client";

import { useEffect, useRef } from "react";

// Calls refresh() every `ms` while the tab is visible, and right away when
// the user comes back to the tab. Live WebSocket events stay the main source
// of updates; this catches anything they missed (a reconnect, a sleeping
// laptop) and things the backend does not push, such as new feed posts.
export default function useAutoRefresh(refresh, ms = 20000, enabled = true) {
  const latest = useRef(refresh);
  latest.current = refresh;

  useEffect(() => {
    if (!enabled) return;
    const run = () => {
      if (document.visibilityState === "visible") latest.current();
    };
    const timer = setInterval(run, ms);
    document.addEventListener("visibilitychange", run);
    window.addEventListener("focus", run);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", run);
      window.removeEventListener("focus", run);
    };
  }, [ms, enabled]);
}
