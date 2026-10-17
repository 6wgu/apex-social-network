"use client";

import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";

export default function LoginPage() {
  const { setUser } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      // Setting the user makes AuthGate send us to the home page.
      setUser(await api("/api/login", { method: "POST", body: { email, password }, redirectOn401: false }));
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  }

  return (
    <div className="auth-layout">
      <aside className="auth-aside auth-aside-login">
        <div>
          <p className="eyebrow">Welcome back</p>
          <h2>Your people are waiting.</h2>
          <p>Pick up the conversations, updates, and little moments you care about.</p>
        </div>
        <div className="auth-aside-mark" aria-hidden="true">↗</div>
      </aside>
      <section className="card auth-card">
        <div className="auth-heading">
          <h1>Log in</h1>
          <p>Return to your circle and see what is happening.</p>
        </div>
        <form onSubmit={submit} className="form auth-form">
          <label>
            Email
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" placeholder="you@example.com" required />
          </label>
          <label>
            Password
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              placeholder="Your password"
              required
            />
          </label>
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <button className="btn-primary" disabled={busy}>
            {busy ? "Logging in…" : "Log in"}
          </button>
        </form>
        <p className="muted auth-footer">
          New here? <Link href="/register">Create an account</Link>
        </p>
      </section>
    </div>
  );
}
