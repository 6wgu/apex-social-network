"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import Icon from "@/components/Icon";
import ImagePicker from "@/components/ImagePicker";

export const PRIVACY = {
  public: { icon: "globe", label: "Public", hint: "Everyone can see it (only your followers while your profile is private)" },
  almost_private: { icon: "groups", label: "Almost private", hint: "Only your followers" },
  private: { icon: "lock", label: "Private", hint: "Only the followers you choose" },
};

// Form for POST /api/posts: text, image/GIF with preview, privacy, and, for
// Private only, a picker of the followers who may see the post. The server
// checks everything again (e.g. that chosen users really follow you).
// With groupId it posts to that group instead (POST /api/groups/{id}/posts):
// a group post has no privacy choice, its group's members see it.
export default function PostComposer({ onCreated, groupId }) {
  const { user } = useAuth();
  const [content, setContent] = useState("");
  const [image, setImage] = useState(null);
  const [privacy, setPrivacy] = useState("public");
  const [followers, setFollowers] = useState(null);
  const [chosen, setChosen] = useState([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // The follower list is only needed once Private is picked.
  useEffect(() => {
    if (privacy !== "private" || followers) return;
    api(`/api/users/${user.id}/followers`)
      .then(setFollowers)
      .catch((err) => setError(err.message));
  }, [privacy, followers, user.id]);

  function toggle(id) {
    setChosen((c) => (c.includes(id) ? c.filter((x) => x !== id) : [...c, id]));
  }

  async function submit(e) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const form = new FormData();
    form.append("content", content);
    if (!groupId) {
      form.append("privacy", privacy);
      if (privacy === "private") chosen.forEach((id) => form.append("viewer_ids", id));
    }
    if (image) form.append("image", image);
    try {
      const post = await api(groupId ? `/api/groups/${groupId}/posts` : "/api/posts", { method: "POST", body: form });
      setContent("");
      setImage(null);
      setChosen([]);
      onCreated(post);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  const empty = !content.trim() && !image;
  const needViewers = !groupId && privacy === "private" && chosen.length === 0;

  return (
    <form className="card composer" onSubmit={submit}>
      <div className="composer-top">
        <Avatar user={user} size={40} />
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder={groupId ? `Write something to the group, ${user.first_name}…` : `What's on your mind, ${user.first_name}?`}
          rows={3}
          maxLength={5000}
          aria-label="Post text"
        />
      </div>

      <ImagePicker value={image} onChange={setImage} onError={setError} />

      {groupId ? (
        <p className="hint">Only members of this group can see this post.</p>
      ) : (
        <>
          <fieldset className="privacy-picker">
            <legend>Who can see this?</legend>
            {Object.entries(PRIVACY).map(([value, p]) => (
              <label key={value} className={privacy === value ? "selected" : ""} title={p.hint}>
                <input type="radio" name="privacy" value={value} checked={privacy === value} onChange={() => setPrivacy(value)} />
                <Icon name={p.icon} size={16} /> {p.label}
              </label>
            ))}
          </fieldset>
          <p className="hint">{PRIVACY[privacy].hint}.</p>

          {privacy === "private" && (
            <div className="viewer-picker">
              {!followers && <p className="muted">Loading your followers…</p>}
              {followers && followers.length === 0 && (
                <p className="muted">You have no followers yet, so there is nobody to share a private post with.</p>
              )}
              {followers && followers.length > 0 && (
                <>
                  <p className="hint">
                    Choose followers ({chosen.length} of {followers.length} selected):
                  </p>
                  <ul>
                    {followers.map((f) => (
                      <li key={f.id}>
                        <label className={chosen.includes(f.id) ? "selected" : ""}>
                          <input type="checkbox" checked={chosen.includes(f.id)} onChange={() => toggle(f.id)} />
                          <Avatar user={f} size={28} />
                          <span className="viewer-name">
                            {f.first_name} {f.last_name}
                          </span>
                        </label>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </div>
          )}
        </>
      )}

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="form-actions">
        <button className="btn-primary" disabled={busy || empty || needViewers}>
          {busy ? "Posting…" : "Post"}
        </button>
      </div>
    </form>
  );
}
