"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { emailProblem, sentence } from "@/lib/validation";
import { useAuth } from "@/components/AuthProvider";
import ConfirmDialog from "@/components/ConfirmDialog";
import Field from "@/components/Field";

// The seconds a 429 asks to wait (its message names them), or a minute.
const secondsToWait = (err) => Number(err.message.match(/\d+/)?.[0] || 60);

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
  // "Wrong email? Change it": whether the form is open, what is typed in
  // it and what is wrong with that.
  const [editing, setEditing] = useState(false);
  const [newEmail, setNewEmail] = useState("");
  const [emailError, setEmailError] = useState("");
  const [changing, setChanging] = useState(false);
  // "Start over": the dialog that asks first, and the delete under way.
  const [confirming, setConfirming] = useState(false);
  const [deleting, setDeleting] = useState(false);

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

  // The server's rules and messages: the email format of the register page,
  // and not the address the account already has.
  function newEmailProblem(value) {
    const email = value.trim().toLowerCase();
    return emailProblem(email) || (email === user.email ? "that is already your email" : "");
  }

  function openEmailForm() {
    setNewEmail("");
    setEmailError("");
    setEditing(true);
  }

  // The server checks the address again (a taken one is a 409, as on the
  // register page), sends a new code there and only then makes it the
  // account's email. The code sent to the old address stops working.
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

  // Deletes this account, which is not verified yet, so its email can be
  // registered again, and leaves for the register page logged out.
  async function startOver() {
    setDeleting(true);
    try {
      await api("/api/me", { method: "DELETE" });
    } catch (err) {
      setDeleting(false);
      setConfirming(false);
      setEditing(false);
      setError(err.message);
      return;
    }
    leave("/register");
  }

  if (user?.is_verified) {
    return (
      <div className="auth-screen">
        <section className="card auth-card verify-card verify-card-done">
          {/* the circle draws itself, then the tick, then a ring pulses out */}
          <div className="verify-success" aria-hidden="true">
            <span className="verify-ring" />
            {Array.from({ length: 8 }, (_, i) => (
              <span key={i} className="verify-spark" style={{ "--i": i }} />
            ))}
            <svg className="verify-check" viewBox="0 0 64 64">
              <circle className="verify-check-circle" cx="32" cy="32" r="29" />
              <path className="verify-check-tick" d="M19 33.5 28 42.5 45.5 23" />
            </svg>
          </div>
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
        <button type="button" className="link-button" onClick={logout}>
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
        onClose={() => setConfirming(false)}
      />
    )}
    </div>
  );
}
