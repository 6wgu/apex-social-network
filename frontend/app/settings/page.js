"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { IMAGE_ACCEPT, imageProblem } from "@/lib/images";
import useNicknameCheck from "@/lib/useNicknameCheck";
import { NICKNAME_TAKEN, nicknameProblem, sentence } from "@/lib/validation";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import Field from "@/components/Field";
import Icon from "@/components/Icon";
import RacingLights from "@/components/RacingLights";
import { CursorSetting } from "@/components/RacingCursor";
import { notify } from "@/components/Toaster";

const ABOUT_MAX = 500;

function SectionHead({ icon, title, children }) {
  return (
    <div className="settings-head">
      <span className="settings-head-icon">
        <Icon name={icon} size={18} />
      </span>
      <h2>{title}</h2>
      {children}
    </div>
  );
}

// /settings: your own profile details. Nickname, about me and the avatar are
// saved together with PATCH /api/me; the privacy switch saves at once with
// PATCH /api/me/privacy. Name, email and date of birth are set at sign-up
// and cannot be changed through the API, so they are shown read-only.
export default function SettingsPage() {
  const router = useRouter();
  const { user, setUser, logout } = useAuth();
  const [nickname, setNickname] = useState(user.nickname || "");
  const [aboutMe, setAboutMe] = useState(user.about_me || "");
  const [file, setFile] = useState(null);
  const [preview, setPreview] = useState("");
  const [removeAvatar, setRemoveAvatar] = useState(false);
  const [error, setError] = useState("");
  const [nicknameServerError, setNicknameServerError] = useState(""); // the server's answer about the nickname (409)
  const nicknameCheck = useNicknameCheck(nickname, user.nickname || ""); // my own nickname is never checked
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [privacyBusy, setPrivacyBusy] = useState(false);
  const fileInput = useRef(null);

  useEffect(() => {
    if (!file) {
      setPreview("");
      return;
    }
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  const dirty = nickname !== (user.nickname || "") || aboutMe !== (user.about_me || "") || Boolean(file) || removeAvatar;

  // A changed nickname must have the right format and be free; the unchanged
  // one is mine and needs no check.
  const nicknameChanged = nickname.trim() !== (user.nickname || "");
  const nicknameError =
    sentence(nicknameServerError) ||
    sentence(nicknameChanged ? nicknameProblem(nickname.trim()) : "") ||
    (nicknameCheck.status === "taken" ? NICKNAME_TAKEN : "");
  const nicknameFree = !nicknameError && nicknameCheck.status === "available";

  function reset() {
    setNickname(user.nickname || "");
    setAboutMe(user.about_me || "");
    setFile(null);
    setRemoveAvatar(false);
    setError("");
    setNicknameServerError("");
    if (fileInput.current) fileInput.current.value = "";
  }

  function chooseFile(e) {
    const selected = e.target.files[0] || null;
    const problem = selected ? imageProblem(selected) : "";
    if (problem) {
      e.target.value = "";
      setFile(null);
      setError(`Avatar ${problem}.`);
      return;
    }
    setError("");
    setSaved(false);
    setFile(selected);
    setRemoveAvatar(false);
  }

  async function save(e) {
    e.preventDefault();
    if (!dirty || nicknameError) return;
    setError("");
    setSaved(false);
    setBusy(true);
    const form = new FormData();
    form.append("nickname", nickname);
    form.append("about_me", aboutMe);
    if (file) form.append("avatar", file);
    if (removeAvatar) form.append("remove_avatar", "1");
    try {
      setUser(await api("/api/me", { method: "PATCH", body: form }));
      setFile(null);
      setRemoveAvatar(false);
      if (fileInput.current) fileInput.current.value = "";
      setSaved(true);
      notify({ tone: "success", message: "Profile saved." });
    } catch (err) {
      // A nickname problem (taken: 409, or its format) shows under the field.
      if (err.fields?.nickname) setNicknameServerError(err.fields.nickname);
      else setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function togglePrivacy(e) {
    setPrivacyBusy(true);
    setError("");
    try {
      setUser(await api("/api/me/privacy", { method: "PATCH", body: { is_public: !e.target.checked } }));
    } catch (err) {
      setError(err.message);
    } finally {
      setPrivacyBusy(false);
    }
  }

  // What the avatar will look like after saving.
  const shown = removeAvatar ? { ...user, avatar_path: "" } : user;
  const aboutLength = [...aboutMe].length;

  return (
    <form className="settings-page" onSubmit={save}>
      <header className="settings-top">
        <button type="button" className="settings-back" onClick={() => router.back()}>
          <Icon name="arrowLeft" size={17} />
          Back
        </button>
        <div className="settings-title">
          <h1>Profile Settings</h1>
          <p>Manage your presence and account</p>
        </div>
        <div className="settings-actions">
          <button type="button" className="btn-secondary" onClick={reset} disabled={busy || !dirty}>
            Cancel
          </button>
          <button className="btn-primary" disabled={busy || !dirty || aboutLength > ABOUT_MAX || Boolean(nicknameError)}>
            {busy ? (
              <>
                <RacingLights small /> Saving…
              </>
            ) : (
              "Save Changes"
            )}
          </button>
        </div>
      </header>

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {saved && !dirty && (
        <p className="form-info" role="status">
          Your changes are saved.
        </p>
      )}

      <div className="settings-grid">
        <div className="settings-main">
          <section className="card settings-card settings-card-accent">
            <SectionHead icon="info" title="General Info" />
            <div className="form settings-form">
              <div className="form-row">
                <Field label="First name">
                  <input value={user.first_name} readOnly aria-readonly="true" className="is-readonly" />
                </Field>
                <Field label="Last name">
                  <input value={user.last_name} readOnly aria-readonly="true" className="is-readonly" />
                </Field>
              </div>
              <div className="form-row">
                <Field
                  label="Nickname"
                  hint="Start with a letter; letters, numbers, _ or - (up to 30)."
                  error={nicknameError}
                  below={nicknameFree && <span className="field-ok" role="status">✓ Available</span>}
                >
                  <input
                    value={nickname}
                    onChange={(e) => {
                      setNickname(e.target.value);
                      setNicknameServerError("");
                      setSaved(false);
                    }}
                    onBlur={nicknameCheck.checkNow}
                    maxLength={30}
                    placeholder="Optional"
                    aria-invalid={nicknameError ? true : undefined}
                  />
                </Field>
                <Field label="Date of birth">
                  <input value={user.date_of_birth} readOnly aria-readonly="true" className="is-readonly" />
                </Field>
              </div>
              <Field
                label="Email address"
                trailing={
                  user.is_verified ? (
                    <span className="field-badge field-badge-ok">Verified</span>
                  ) : (
                    <Link href="/verify" className="field-badge field-badge-alert">
                      Verify
                    </Link>
                  )
                }
              >
                <input value={user.email} readOnly aria-readonly="true" className="is-readonly" />
              </Field>
              <p className="hint settings-note">
                <Icon name="lock" size={13} /> Name, email and date of birth are set when you sign up and can&apos;t be changed here.
              </p>
            </div>
          </section>

          <section className="card settings-card">
            <div className="setting-row">
              <span className="settings-head-icon">
                <Icon name="shield" size={18} />
              </span>
              <span className="setting-row-text">
                <strong>Profile Privacy</strong>
                <span className="muted">When private, only people you approve can see your profile and posts.</span>
              </span>
              <label className="switch">
                <input
                  type="checkbox"
                  checked={!user.is_public}
                  onChange={togglePrivacy}
                  disabled={privacyBusy}
                  aria-label="Private profile"
                />
                <span className="switch-track" aria-hidden="true" />
              </label>
            </div>
          </section>

          <section className="card settings-card">
            <SectionHead icon="file" title="About You">
              <span className={`count-badge${aboutLength > ABOUT_MAX ? " count-badge-over" : ""}`}>
                {aboutLength} / {ABOUT_MAX}
              </span>
            </SectionHead>
            <textarea
              className="settings-about"
              value={aboutMe}
              onChange={(e) => {
                setAboutMe(e.target.value);
                setSaved(false);
              }}
              rows={5}
              maxLength={ABOUT_MAX}
              placeholder="Share something about yourself…"
              aria-label="About you"
            />
          </section>

          <section className="card settings-card">
            <CursorSetting />
          </section>
        </div>

        <aside className="settings-side">
          <section className="card settings-card avatar-card">
            <h2 className="side-heading">Avatar</h2>
            <div className="avatar-card-preview">
              <Avatar user={shown} src={preview} size={132} />
              <button type="button" className="avatar-card-edit" onClick={() => fileInput.current?.click()} aria-label="Choose a new photo">
                <Icon name="edit" size={16} />
              </button>
            </div>
            <p className="hint">JPEG, PNG, GIF or WebP, up to 50 MB.</p>
            <input ref={fileInput} type="file" accept={IMAGE_ACCEPT} onChange={chooseFile} className="visually-hidden" tabIndex={-1} />
            <button type="button" className="btn-secondary btn-block" onClick={() => fileInput.current?.click()}>
              Upload New Photo
            </button>
            {user.avatar_path && (
              <label className="checkbox">
                <input
                  type="checkbox"
                  checked={removeAvatar}
                  onChange={(e) => {
                    setRemoveAvatar(e.target.checked);
                    setSaved(false);
                    if (e.target.checked) {
                      setFile(null);
                      if (fileInput.current) fileInput.current.value = "";
                    }
                  }}
                />
                Remove my current photo
              </label>
            )}
          </section>

          <section className="danger-zone">
            <h2>Danger Zone</h2>
            <p>Sign out of APEX on this device. You will need your password to sign back in.</p>
            <button type="button" className="btn-danger btn-block" onClick={logout}>
              <Icon name="logout" size={17} />
              Log out
            </button>
          </section>
        </aside>
      </div>
    </form>
  );
}
