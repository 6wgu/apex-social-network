"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";

const AVATAR_ACCEPT = "image/jpeg,image/png,image/gif,.jpg,.jpeg,.png,.gif";
const AVATAR_TYPES = new Set(["image/jpeg", "image/png", "image/gif"]);

function isAllowedAvatar(file) {
  return file && AVATAR_TYPES.has(file.type) && /\.(jpe?g|png|gif)$/i.test(file.name);
}

export default function RegisterPage() {
  const { setUser } = useAuth();
  const [avatar, setAvatar] = useState(null);
  const [preview, setPreview] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

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
    try {
      // The server validates everything and its message is shown as is.
      // Setting the user makes AuthGate send us to the home page.
      setUser(await api("/api/register", { method: "POST", body: form, redirectOn401: false }));
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  }

  function chooseAvatar(e) {
    const file = e.target.files[0] || null;
    if (file && !isAllowedAvatar(file)) {
      e.target.value = "";
      setAvatar(null);
      setError("Avatar must be a JPEG, PNG or GIF file.");
      return;
    }
    setError("");
    setAvatar(file);
  }

  return (
    <section className="card auth-card">
      <h1>Create an account</h1>
      <form onSubmit={submit} className="form" noValidate>
        <div className="form-row">
          <label>
            First name
            <input name="first_name" autoComplete="given-name" required />
          </label>
          <label>
            Last name
            <input name="last_name" autoComplete="family-name" required />
          </label>
        </div>
        <label>
          Email
          <input name="email" type="email" autoComplete="email" required />
        </label>
        <label>
          Password
          <input name="password" type="password" autoComplete="new-password" required />
          <span className="hint">At least 8 characters.</span>
        </label>
        <label>
          Date of birth
          <input name="date_of_birth" type="date" required />
        </label>

        <fieldset className="optional">
          <legend>Optional</legend>
          <div className="avatar-field">
            <Avatar src={preview} size={64} />
            <label>
              Avatar
              <input
                name="avatar"
                type="file"
                accept={AVATAR_ACCEPT}
                onChange={chooseAvatar}
              />
              <span className="hint">JPEG, PNG or GIF, up to 5 MB.</span>
            </label>
          </div>
          <label>
            Nickname
            <input name="nickname" maxLength={30} />
            <span className="hint">Start with a letter; use only letters, numbers, _ or - (up to 30 characters).</span>
          </label>
          <label>
            About me
            <textarea name="about_me" rows={3} maxLength={500} />
          </label>
        </fieldset>

        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <button className="btn-primary" disabled={busy}>
          {busy ? "Creating account…" : "Register"}
        </button>
      </form>
      <p className="muted">
        Already have an account? <Link href="/login">Log in</Link>
      </p>
    </section>
  );
}
