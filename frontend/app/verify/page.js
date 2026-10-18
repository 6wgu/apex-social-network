"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";

export default function VerifyPage() {
  const { user, setUser } = useAuth();
  const [code, setCode] = useState("");
  const [sentTo, setSentTo] = useState("");
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);
  const [sending, setSending] = useState(false);
  const [wait, setWait] = useState(0);
  const autoSent = useRef(false);

  useEffect(() => {
    if (wait <= 0) return;
    const t = setTimeout(() => setWait((w) => w - 1), 1000);
    return () => clearTimeout(t);
  }, [wait]);

  const sendCode = useCallback(async () => {
    setError("");
    setInfo("");
    setSending(true);
    try {
      const res = await api("/api/otp/send", { method: "POST", body: {} });
      setSentTo(res.email);
      setInfo(`We sent a code to ${res.email}.`);
      setWait(60);
    } catch (err) {
      if (err instanceof ApiError && err.status === 429) {
        const seconds = Number(err.message.match(/\d+/)?.[0] || 60);
        setWait(seconds);
        setInfo("A code was sent a moment ago. Check your inbox.");
      } else if (err instanceof ApiError && err.status === 409) {
        setUser({ ...user, is_verified: true });
      } else {
        setError(err.message);
      }
    } finally {
      setSending(false);
    }
  }, [setUser, user]);

  useEffect(() => {
    if (!user || user.is_verified || autoSent.current) return;
    autoSent.current = true;
    sendCode();
  }, [user, sendCode]);

  async function submit(e) {
    e.preventDefault();
    setError("");
    setInfo("");
    const clean = code.replace(/\D/g, "");
    if (clean.length !== 6) {
      setError("Enter the 6-digit code from the email.");
      return;
    }
    setBusy(true);
    try {
      setUser(await api("/api/otp/verify", { method: "POST", body: { code: clean } }));
    } catch (err) {
      setError(err.message);
      setCode("");
    } finally {
      setBusy(false);
    }
  }

  if (user?.is_verified) {
    return (
      <section className="card auth-card verify-card">
        <div className="verify-mark verify-mark-done" aria-hidden="true">✓</div>
        <div className="auth-heading">
          <h1>Email verified</h1>
          <p>{user.email} is confirmed. You are all set.</p>
        </div>
        <Link href="/" className="btn-primary verify-home">
          Go to your feed
        </Link>
      </section>
    );
  }

  return (
    <section className="card auth-card verify-card">
      <div className="verify-mark" aria-hidden="true">✉</div>
      <div className="auth-heading">
        <h1>Verify your email</h1>
        <p>Enter the 6-digit code we sent to {sentTo || user?.email}. It expires in 10 minutes.</p>
      </div>
      <form onSubmit={submit} className="form auth-form" noValidate>
        <label>
          Verification code
          <input
            className="otp-input"
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
            inputMode="numeric"
            autoComplete="one-time-code"
            placeholder="000000"
            maxLength={6}
            autoFocus
          />
        </label>
        {info && !error && (
          <p className="form-info" role="status">
            {info}
          </p>
        )}
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <button className="btn-primary" disabled={busy || code.length !== 6}>
          {busy ? "Checking…" : "Verify"}
        </button>
      </form>
      <p className="muted auth-footer">
        Didn&apos;t get it?{" "}
        <button type="button" className="link-button" onClick={sendCode} disabled={sending || wait > 0}>
          {sending ? "Sending…" : wait > 0 ? `Send a new code in ${wait}s` : "Send a new code"}
        </button>
      </p>
    </section>
  );
}
