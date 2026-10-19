"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";
import Field, { PasswordField } from "@/components/Field";
import Icon from "@/components/Icon";
import RacingLights from "@/components/RacingLights";
import LaunchOverlay, { GO_MS, MORPH_MS } from "@/components/LaunchOverlay";

const AVATAR_ACCEPT = "image/jpeg,image/png,image/gif,image/webp,.jpg,.jpeg,.png,.gif,.webp";
const AVATAR_TYPES = new Set(["image/jpeg", "image/png", "image/gif", "image/webp"]);

const pause = (ms) => new Promise((done) => setTimeout(done, ms));

function isAllowedAvatar(file) {
  return file && AVATAR_TYPES.has(file.type) && /\.(jpe?g|png|gif|webp)$/i.test(file.name);
}

export default function RegisterPage() {
  const { setUser } = useAuth();
  const [avatar, setAvatar] = useState(null);
  const [preview, setPreview] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [phase, setPhase] = useState(null); // the launch animation, see LaunchOverlay

  // Show the chosen picture before it is uploaded.
  useEffect(() => {
    if (!avatar) {
      setPreview("");
      return;
    }
    const url = URL.createObjectURL(avatar);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [avatar]);

  async function submit(e) {
    e.preventDefault();
    setError("");
    const nickname = String(new FormData(e.currentTarget).get("nickname") || "").trim();
    if (nickname.length > 30) {
      setError("nickname must be 30 characters or fewer");
      return;
    }
    if (nickname && !/^[A-Za-z][A-Za-z0-9_-]*$/.test(nickname)) {
      setError("nickname must start with a letter and contain only letters, numbers, underscores, or hyphens");
      return;
    }
    setBusy(true);
    const form = new FormData(e.target);
    if (!avatar) form.delete("avatar"); // an empty file input still sends a part
    const calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!calm) setPhase("wait");
    try {
      // The server validates everything and its message is shown as is.
      const user = await api("/api/register", { method: "POST", body: form, redirectOn401: false });
      if (!calm) {
        setPhase("morph");
        await pause(MORPH_MS);
        setPhase("go");
        await pause(GO_MS);
      }
      // Setting the user makes AuthGate send us on (home, or verification).
      setUser(user);
    } catch (err) {
      setPhase(null);
      setError(err.message);
      setBusy(false);
    }
  }

  function chooseAvatar(e) {
    const file = e.target.files[0] || null;
    if (file && !isAllowedAvatar(file)) {
      e.target.value = "";
      setAvatar(null);
      setError("Avatar must be a JPEG, PNG, GIF or WebP file.");
      return;
    }
    setError("");
    setAvatar(file);
  }

  return (
    <div className="auth-screen">
      <LaunchOverlay phase={phase} />
      <section className="card auth-card auth-card-wide">
        <div className="auth-heading">
          <h1>Create your account</h1>
          <p>Your corner of the network starts here.</p>
        </div>
        <form onSubmit={submit} className="form auth-form" noValidate>
          <div className="form-row">
            <Field label="First name" icon="user">
              <input name="first_name" autoComplete="given-name" placeholder="Alex" required />
            </Field>
            <Field label="Last name" icon="user">
              <input name="last_name" autoComplete="family-name" placeholder="Rivers" required />
            </Field>
          </div>
          <Field label="Email address" icon="mail">
            <input name="email" type="email" autoComplete="email" placeholder="alex@example.com" required />
          </Field>
          <div className="form-row">
            <PasswordField name="password" autoComplete="new-password" placeholder="8+ characters" required />
            <Field label="Date of birth" icon="calendar">
              <input name="date_of_birth" type="date" required />
            </Field>
          </div>

          <fieldset className="optional">
            <legend>
              Profile customization <span className="optional-tag">(Optional)</span>
            </legend>
            <div className="customize">
              <label className={`avatar-drop${preview ? " has-image" : ""}`} title="Choose an avatar">
                <input name="avatar" type="file" accept={AVATAR_ACCEPT} onChange={chooseAvatar} className="visually-hidden" />
                {preview ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img src={preview} alt="Chosen avatar" />
                ) : (
                  <>
                    <Icon name="camera" size={20} />
                    <span>Avatar</span>
                  </>
                )}
              </label>
              <div className="customize-fields">
                <Field icon="user" hint="Start with a letter; letters, numbers, _ or - (up to 30).">
                  <input name="nickname" maxLength={30} placeholder="Nickname (@handle)" aria-label="Nickname" />
                </Field>
                <Field icon="message">
                  <textarea name="about_me" rows={1} maxLength={500} placeholder="Tell us about yourself…" aria-label="About me" />
                </Field>
              </div>
            </div>
            <p className="hint">Avatar: JPEG, PNG, GIF or WebP, up to 50 MB.</p>
          </fieldset>

          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <button className="btn-primary btn-block" disabled={busy}>
            {busy ? (
              <>
                <RacingLights small /> Creating account…
              </>
            ) : (
              "Complete registration"
            )}
          </button>
        </form>
        <p className="muted auth-footer">
          Already have an account? <Link href="/login">Log in</Link>
        </p>
      </section>
    </div>
  );
}
