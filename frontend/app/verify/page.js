"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { emailProblem, sentence } from "@/lib/validation";
import { useAuth } from "@/components/AuthProvider";
import ConfirmDialog from "@/components/ConfirmDialog";
import Field from "@/components/Field";
import SuccessCheck from "@/components/SuccessCheck";

const secondsToWait = (err) => Number(err.message.match(/\d+/)?.[0] || 60);

const GUARD = "verifyBackGuard";

function useBackGuard(active, onBack) {
  const onTop = useRef(false);
  const stepping = useRef(null);
  const onBackRef = useRef(onBack);
  onBackRef.current = onBack;

  useEffect(() => {
    if (!active) return;
    if (!onTop.current) {
      if (!window.history.state?.[GUARD]) window.history.pushState({ [GUARD]: true }, "");
      onTop.current = true;
    }
    const onPop = () => {
      onTop.current = false;
      if (stepping.current) {
        stepping.current();
        stepping.current = null;
      } else {
        onBackRef.current();
      }
    };
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, [active]);

  return useMemo(
    () => ({
      restore() {
        if (onTop.current) return;
        window.history.pushState({ [GUARD]: true }, "");
        onTop.current = true;
      },
      remove() {
        if (!onTop.current) return Promise.resolve();
        return new Promise((done) => {
          stepping.current = done;
          window.history.back();
        });
      },
    }),
    []
  );
}

export default function VerifyPage() {
  const { user, setUser, logout, leave } = useAuth();
  const [code, setCode] = useState("");
  const [sentTo, setSentTo] = useState("");
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);
  const [sending, setSending] = useState(false);
  const [wait, setWait] = useState(0);
  const autoSent = useRef(false);
  const [editing, setEditing] = useState(false);
  const [newEmail, setNewEmail] = useState("");
  const [emailError, setEmailError] = useState("");
  const [changing, setChanging] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const guard = useBackGuard(Boolean(user) && !user.is_verified, () => setConfirming(true));

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
        setWait(secondsToWait(err));
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

  function newEmailProblem(value) {
    const email = value.trim().toLowerCase();
    return emailProblem(email) || (email === user.email ? "that is already your email" : "");
  }

  function openEmailForm() {
    setNewEmail("");
    setEmailError("");
    setEditing(true);
  }

  async function changeEmail(e) {
    e.preventDefault();
    const problem = newEmailProblem(newEmail);
    if (problem) {
      setEmailError(problem);
      return;
    }
    setChanging(true);
    try {
      const updated = await api("/api/me/email", { method: "PATCH", body: { email: newEmail.trim().toLowerCase() } });
      setUser(updated);
      setSentTo(updated.email);
      setEditing(false);
      setCode("");
      setError("");
      setInfo(`We sent a new code to ${updated.email}.`);
      setWait(60);
    } catch (err) {
      if (err instanceof ApiError && err.status === 429) setWait(secondsToWait(err));
      else if (err instanceof ApiError && err.status === 403) setUser({ ...user, is_verified: true });
      else setEmailError(err.fields?.email || err.message);
    } finally {
      setChanging(false);
    }
  }

  function stay() {
    setConfirming(false);
    guard.restore();
  }

  async function startOver() {
    setDeleting(true);
    try {
      await api("/api/me", { method: "DELETE" });
    } catch (err) {
      setDeleting(false);
      stay();
      setEditing(false);
      setError(err.message);
      return;
    }
    await guard.remove();
    leave("/register");
  }

  async function signOut() {
    await guard.remove();
    logout();
  }

  if (user?.is_verified) {
    return (
      <div className="auth-screen">
        <section className="card auth-card verify-card verify-card-done">
          <SuccessCheck />
          <div className="auth-heading verify-done-text">
            <h1>Email verified</h1>
            <p>{user.email} is confirmed. You are all set.</p>
          </div>
          <Link href="/" className="btn-primary verify-home verify-done-text">
            Go to your feed
          </Link>
        </section>
      </div>
    );
  }

  const email = sentTo || user?.email;
  return (
    <div className="auth-screen">
    <section className="card auth-card verify-card">
      <div className="verify-mark" aria-hidden="true">✉</div>
      {editing ? (
        <>
          <div className="auth-heading">
            <h1>Change your email</h1>
            <p>
              We will send a new code to the new address, and the code sent to <strong>{email}</strong> will stop
              working.
            </p>
          </div>
          <form onSubmit={changeEmail} className="form auth-form" noValidate>
            <Field
              label="New email address"
              icon="mail"
              error={sentence(emailError)}
              hint={wait > 0 ? `A new code can be sent in ${wait}s.` : null}
            >
              <input
                type="email"
                value={newEmail}
                onChange={(e) => {
                  setNewEmail(e.target.value);
                  if (emailError) setEmailError(newEmailProblem(e.target.value));
                }}
                onBlur={() => newEmail.trim() && setEmailError(newEmailProblem(newEmail))}
                autoComplete="email"
                placeholder="alex@example.com"
                maxLength={254}
                aria-invalid={emailError ? true : undefined}
                autoFocus
                required
              />
            </Field>
            <button className="btn-primary" disabled={changing || wait > 0}>
              {changing ? "Sending…" : "Send a new code"}
            </button>
          </form>
          <p className="muted auth-footer">
            <button type="button" className="link-button" onClick={() => setEditing(false)}>
              Cancel
            </button>
          </p>
        </>
      ) : (
        <>
          <div className="auth-heading">
            <h1>Verify your email</h1>
            <p>
              Enter the 6-digit code we sent to <strong>{email}</strong>. It expires in 10 minutes.
            </p>
            <p className="verify-change">
              Wrong email?{" "}
              <button type="button" className="link-button" onClick={openEmailForm}>
                Change it
              </button>
            </p>
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
        </>
      )}
      <div className="verify-exit">
        <button type="button" className="btn-secondary btn-small" onClick={() => setConfirming(true)}>
          Start over
        </button>
        <button type="button" className="link-button" onClick={signOut}>
          Log out
        </button>
      </div>
    </section>
    {confirming && (
      <ConfirmDialog
        title="Start over?"
        message="This will delete the account you just created, so you can register again with this email or another one."
        confirmLabel="Delete and start over"
        busy={deleting}
        onConfirm={startOver}
        onClose={stay}
      />
    )}
    </div>
  );
}
