"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import Avatar from "@/components/Avatar";

const AVATAR_ACCEPT = "image/jpeg,image/png,image/gif,.jpg,.jpeg,.png,.gif";
const AVATAR_TYPES = new Set(["image/jpeg", "image/png", "image/gif"]);

function isAllowedAvatar(file) {
  return file && AVATAR_TYPES.has(file.type) && /\.(jpe?g|png|gif)$/i.test(file.name);
}

// Form for PATCH /api/me on your own profile: nickname, about me, and the
// avatar (replace it with a new picture, or remove it). The server validates
// everything; its error message is shown as is.
export default function EditProfile({ user, onSaved, onCancel }) {
  const [nickname, setNickname] = useState(user.nickname);
  const [aboutMe, setAboutMe] = useState(user.about_me);
  const [file, setFile] = useState(null);
  const [preview, setPreview] = useState("");
  const [removeAvatar, setRemoveAvatar] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // Show a newly chosen picture before it is uploaded.
  useEffect(() => {
    if (!file) {
      setPreview("");
      return;
    }
    const url = URL.createObjectURL(file);
    setPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);

  function chooseFile(e) {
    const selected = e.target.files[0] || null;
    if (selected && !isAllowedAvatar(selected)) {
      e.target.value = "";
      setFile(null);
      setError("Avatar must be a JPEG, PNG or GIF file.");
      return;
    }
    setError("");
    setFile(selected);
    setRemoveAvatar(false);
  }

  function toggleRemove(e) {
    setRemoveAvatar(e.target.checked);
    if (e.target.checked) {
      setFile(null);
      document.getElementById("edit-avatar").value = "";
    }
  }

  async function submit(e) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const form = new FormData();
    form.append("nickname", nickname);
    form.append("about_me", aboutMe);
    if (file) form.append("avatar", file);
    if (removeAvatar) form.append("remove_avatar", "1");
    try {
      onSaved(await api("/api/me", { method: "PATCH", body: form }));
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  }

  // What the avatar will look like after saving.
  const shown = removeAvatar ? { ...user, avatar_path: "" } : user;

  return (
    <section className="card">
      <h2>Edit profile</h2>
      <form onSubmit={submit} className="form">
        <div className="avatar-field">
          <Avatar user={shown} src={preview} size={64} />
          <label>
            Avatar
            <input id="edit-avatar" type="file" accept={AVATAR_ACCEPT} onChange={chooseFile} />
            <span className="hint">JPEG, PNG or GIF, up to 5 MB.</span>
          </label>
        </div>
        {user.avatar_path && (
          <label className="checkbox">
            <input type="checkbox" checked={removeAvatar} onChange={toggleRemove} />
            Remove my current avatar
          </label>
        )}
        <label>
          Nickname
          <input value={nickname} onChange={(e) => setNickname(e.target.value)} maxLength={30} />
        </label>
        <label>
          About me
          <textarea value={aboutMe} onChange={(e) => setAboutMe(e.target.value)} rows={3} maxLength={500} />
        </label>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <div className="form-actions">
          <button type="button" className="btn-secondary" onClick={onCancel} disabled={busy}>
            Cancel
          </button>
          <button className="btn-primary" disabled={busy}>
            {busy ? "Saving…" : "Save"}
          </button>
        </div>
      </form>
    </section>
  );
}
