"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";

// Phase 0 home page: proves the frontend reaches the backend through the proxy.
// Replaced by the feed in Phase 2.
export default function Home() {
  const [health, setHealth] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api("/api/health")
      .then(setHealth)
      .catch((err) => setError(err.message));
  }, []);

  return (
    <section className="card">
      <h1>Backend status</h1>
      {!health && !error && <p className="muted">Checking…</p>}
      {error && <p className="error">Backend unreachable: {error}</p>}
      {health && (
        <ul className="status-list">
          <li>
            API: <strong className="ok">{health.status}</strong>
          </li>
          <li>
            Database: <strong className={health.database === "ok" ? "ok" : "error"}>{health.database}</strong>
          </li>
        </ul>
      )}
    </section>
  );
}
