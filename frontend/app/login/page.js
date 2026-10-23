"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { handOffEmail, takeEmail } from "@/lib/emailHandoff";
import { useAuth } from "@/components/AuthProvider";
import Field, { PasswordField } from "@/components/Field";
import RacingLights from "@/components/RacingLights";
import LaunchOverlay, { GO_MS, MORPH_MS } from "@/components/LaunchOverlay";

const pause = (ms) => new Promise((done) => setTimeout(done, ms));

export default function LoginPage() {
  const { setUser } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [phase, setPhase] = useState(null);

  useEffect(() => {
    const handed = takeEmail();
    if (handed) setEmail(handed);
  }, []);

  async function submit(e) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!calm) setPhase("wait");
    try {
      const user = await api("/api/login", { method: "POST", body: { email, password }, redirectOn401: false });
      if (!calm) {
        setPhase("morph");
        await pause(MORPH_MS);
        setPhase("go");
        await pause(GO_MS);
      }
      setUser(user);
    } catch (err) {
      setPhase(null);
      setError(err.message);
      setBusy(false);
    }
  }

  return (
    <div className="auth-screen">
      <LaunchOverlay phase={phase} />
      <section className="card auth-card">
        <div className="auth-heading">
          <h1>Welcome back</h1>
          <p>Please enter your details to sign in.</p>
        </div>
        <form onSubmit={submit} className="form auth-form">
          <Field label="Email" icon="mail">
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" placeholder="you@example.com" maxLength={254} required />
          </Field>
          <PasswordField
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
            placeholder="Your password"
            required
          />
          <p className="auth-forgot">
            <Link href="/forgot-password" onClick={() => handOffEmail(email)}>
              Forgot password?
            </Link>
          </p>
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <button className="btn-primary btn-block" disabled={busy}>
            {busy ? (
              <>
                <RacingLights small /> Signing in…
              </>
            ) : (
              "Sign in"
            )}
          </button>
        </form>
        <p className="muted auth-footer">
          Don&apos;t have an account yet? <Link href="/register">Create one here</Link>
        </p>
      </section>
    </div>
  );
}
