"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { handOffEmail, takeEmail } from "@/lib/emailHandoff";
import { emailProblem, passwordProblem, sentence } from "@/lib/validation";
import Field, { PasswordField } from "@/components/Field";
import PasswordRules from "@/components/PasswordRules";
import SuccessCheck from "@/components/SuccessCheck";

// The resend cooldown (OTP_RESEND_SECONDS), as on the verify page.
const RESEND_SECONDS = 60;

// The seconds a 429 asks to wait.
const secondsToWait = (err) => Number(err.body?.retry_after) || RESEND_SECONDS;

const normalize = (email) => email.trim().toLowerCase();

function confirmProblem(password, confirm) {
  if (!confirm) return "type the new password again";
  return confirm === password ? "" : "the passwords don't match";
}

// "Forgot password?": the email, then the code sent to it, then a new
// password. The steps are states of this one page, not separate pages, so
// Back leaves the flow instead of stepping through it (or looping). The
// server answers the same for every address, so the page never says
// whether an account exists.
export default function ForgotPasswordPage() {
  const [step, setStep] = useState("email"); // email, code, password, done
  const [email, setEmail] = useState("");
  const [sentTo, setSentTo] = useState(""); // the address of the code step
  const [code, setCode] = useState("");
  const [token, setToken] = useState(""); // from a correct code, for the reset
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [attempted, setAttempted] = useState(false); // pressed "Change password" once
  const [errors, setErrors] = useState({}); // field -> problem, shown under that field
  const [error, setError] = useState(""); // a problem that belongs to no field
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);
  const [sending, setSending] = useState(false); // a resend under way
  const [wait, setWait] = useState(0);

  // The email typed on the login page, if any.
  useEffect(() => {
    const handed = takeEmail();
    if (handed) setEmail(handed);
  }, []);

  useEffect(() => {
    if (wait <= 0) return;
    const t = setTimeout(() => setWait((w) => w - 1), 1000);
    return () => clearTimeout(t);
  }, [wait]);

  const setFieldError = (field, problem) => setErrors((old) => ({ ...old, [field]: problem }));

  async function sendCode(e) {
    e.preventDefault();
    setError("");
    const address = normalize(email);
    const problem = emailProblem(address);
    if (problem) {
      setErrors({ email: problem });
      return;
    }
    setBusy(true);
    try {
      const res = await api("/api/password/forgot", { method: "POST", body: { email: address } });
      setInfo(res.message);
      setWait(RESEND_SECONDS);
      setSentTo(address);
      setCode("");
      setErrors({});
      setStep("code");
    } catch (err) {
      if (err.status === 429 && address === sentTo && secondsToWait(err) <= RESEND_SECONDS) {
        // Back from "Change email" with the same address: its code is still good.
        setWait(secondsToWait(err));
        setInfo("A code was sent a moment ago. Check your inbox.");
        setErrors({});
        setStep("code");
      } else {
        setErrors({ email: err.fields?.email || err.message });
      }
    } finally {
      setBusy(false);
    }
  }

  async function resend() {
    setError("");
    setInfo("");
    setErrors({});
    setSending(true);
    try {
      const res = await api("/api/password/forgot", { method: "POST", body: { email: sentTo } });
      setInfo(res.message);
      setWait(RESEND_SECONDS);
      setCode("");
    } catch (err) {
      if (err.status === 429) setWait(secondsToWait(err));
      setError(sentence(err.message));
    } finally {
      setSending(false);
    }
  }

  function changeEmail() {
    setEmail(sentTo);
    setErrors({});
    setError("");
    setInfo("");
    setStep("email");
  }

  async function checkCode(e) {
    e.preventDefault();
    setError("");
    setInfo("");
    const clean = code.replace(/\D/g, "");
    if (clean.length !== 6) {
      setErrors({ code: "enter the 6-digit code from the email" });
      return;
    }
    setBusy(true);
    try {
      const res = await api("/api/password/verify-code", { method: "POST", body: { email: sentTo, code: clean } });
      setToken(res.token);
      setErrors({});
      setStep("password");
    } catch (err) {
      // "wrong code, 3 attempts left", or expired / used up: ask for a new one.
      setErrors({ code: err.message });
      setCode("");
    } finally {
      setBusy(false);
    }
  }

  async function changePassword(e) {
    e.preventDefault();
    setError("");
    setAttempted(true);
    const problems = { password: passwordProblem(password), confirm: confirmProblem(password, confirm) };
    if (problems.password || problems.confirm) {
      setErrors(problems);
      return;
    }
    setBusy(true);
    try {
      await api("/api/password/reset", { method: "POST", body: { token, password } });
      setToken("");
      setPassword("");
      setConfirm("");
      setStep("done");
    } catch (err) {
      if (err.body?.reason === "invalid_token") {
        // Too late (or already used): start again from the email.
        setToken("");
        setErrors({});
        setEmail(sentTo);
        setStep("email");
        setError(sentence(err.message));
      } else if (err.fields) {
        setErrors(err.fields);
      } else {
        setError(err.message);
      }
    } finally {
      setBusy(false);
    }
  }

  const shown = (field) => sentence(errors[field]);
  const invalid = (field) => (errors[field] ? true : undefined);
  const formError = error && (
    <p className="form-error" role="alert">
      {error}
    </p>
  );
  const backToLogin = (
    <Link href="/login" replace onClick={() => handOffEmail(sentTo || email)}>
      Back to log in
    </Link>
  );

  if (step === "done") {
    return (
      <div className="auth-screen">
        <section className="card auth-card verify-card verify-card-done">
          <SuccessCheck />
          <div className="auth-heading verify-done-text">
            <h1>Password changed</h1>
            <p>Log in with your new password. Every device that was signed in to your account has been signed out.</p>
          </div>
          <Link href="/login" replace className="btn-primary verify-home verify-done-text" onClick={() => handOffEmail(sentTo)}>
            Log in
          </Link>
        </section>
      </div>
    );
  }

  if (step === "code") {
    return (
      <div className="auth-screen">
        <section className="card auth-card verify-card">
          <div className="verify-mark" aria-hidden="true">✉</div>
          <div className="auth-heading">
            <h1>Check your email</h1>
            <p>
              If an account exists for <strong>{sentTo}</strong>, we sent a 6-digit code to it. It expires in 10 minutes.
            </p>
            <p className="verify-change">
              Wrong email?{" "}
              <button type="button" className="link-button" onClick={changeEmail}>
                Change email
              </button>
            </p>
          </div>
          <form onSubmit={checkCode} className="form auth-form" noValidate>
            <Field label="Reset code" error={shown("code")}>
              <input
                className="otp-input"
                value={code}
                onChange={(e) => {
                  setCode(e.target.value.replace(/\D/g, "").slice(0, 6));
                  if (errors.code) setFieldError("code", "");
                }}
                inputMode="numeric"
                autoComplete="one-time-code"
                placeholder="000000"
                maxLength={6}
                aria-invalid={invalid("code")}
                autoFocus
              />
            </Field>
            {info && !error && !errors.code && (
              <p className="form-info" role="status">
                {info}
              </p>
            )}
            {formError}
            <button className="btn-primary" disabled={busy || code.length !== 6}>
              {busy ? "Checking…" : "Verify code"}
            </button>
          </form>
          <p className="muted auth-footer">
            Didn&apos;t get it?{" "}
            <button type="button" className="link-button" onClick={resend} disabled={sending || wait > 0}>
              {sending ? "Sending…" : wait > 0 ? `Send a new code in ${wait}s` : "Send a new code"}
            </button>
          </p>
        </section>
      </div>
    );
  }

  if (step === "password") {
    return (
      <div className="auth-screen">
        <section className="card auth-card">
          <div className="auth-heading">
            <h1>Choose a new password</h1>
            <p>Every device signed in to your account will be signed out.</p>
          </div>
          <form onSubmit={changePassword} className="form auth-form" noValidate>
            {/* Lets a password manager save the new password for this account. */}
            <input type="email" name="username" autoComplete="username" value={sentTo} readOnly hidden />
            <PasswordField
              label="New password"
              value={password}
              onChange={(e) => {
                setPassword(e.target.value);
                if (errors.password) setFieldError("password", passwordProblem(e.target.value));
                if (errors.confirm && confirm) setFieldError("confirm", confirmProblem(e.target.value, confirm));
              }}
              autoComplete="new-password"
              placeholder="Strong password"
              aria-invalid={invalid("password")}
              error={shown("password")}
              below={<PasswordRules password={password} strict={attempted || Boolean(errors.password)} />}
              autoFocus
            />
            <PasswordField
              label="Confirm new password"
              value={confirm}
              onChange={(e) => {
                setConfirm(e.target.value);
                if (errors.confirm) setFieldError("confirm", confirmProblem(password, e.target.value));
              }}
              autoComplete="new-password"
              placeholder="Type it again"
              aria-invalid={invalid("confirm")}
              error={shown("confirm")}
            />
            {formError}
            <button className="btn-primary btn-block" disabled={busy}>
              {busy ? "Saving…" : "Change password"}
            </button>
          </form>
          <p className="muted auth-footer">Changed your mind? {backToLogin}</p>
        </section>
      </div>
    );
  }

  return (
    <div className="auth-screen">
      <section className="card auth-card">
        <div className="auth-heading">
          <h1>Forgot password?</h1>
          <p>Enter the email of your account and we&apos;ll send you a code to choose a new password.</p>
        </div>
        <form onSubmit={sendCode} className="form auth-form" noValidate>
          <Field label="Email address" icon="mail" error={shown("email")}>
            <input
              type="email"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                if (errors.email) setFieldError("email", emailProblem(normalize(e.target.value)));
              }}
              autoComplete="email"
              placeholder="you@example.com"
              maxLength={254}
              aria-invalid={invalid("email")}
              autoFocus
              required
            />
          </Field>
          {formError}
          <button className="btn-primary btn-block" disabled={busy}>
            {busy ? "Sending…" : "Send code"}
          </button>
        </form>
        <p className="muted auth-footer">Remembered it? {backToLogin}</p>
      </section>
    </div>
  );
}
