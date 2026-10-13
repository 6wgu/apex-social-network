"use client";

import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";

const TABS = ["Posts", "Followers", "Following"];

// /profile/{id}, or /profile/me for the logged-in user.
// What a private profile reveals is decided by the backend: for a profile we
// may not see, GET /api/users/{id} simply does not contain the private fields.
export default function ProfilePage() {
  const params = useParams();
  const { user: me, setUser } = useAuth();
  const id = params.id === "me" ? me.id : params.id;

  const [profile, setProfile] = useState(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState(TABS[0]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setProfile(null);
    setError("");
    api(`/api/users/${encodeURIComponent(id)}`)
      .then(setProfile)
      .catch((err) => setError(err.status === 404 ? "This user does not exist." : err.message));
  }, [id]);

  async function togglePrivacy(e) {
    const isPublic = !e.target.checked; // the switch reads "Private profile"
    setSaving(true);
    setError("");
    try {
      const updated = await api("/api/me/privacy", { method: "PATCH", body: { is_public: isPublic } });
      setProfile((p) => ({ ...p, user: updated }));
      setUser(updated);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  if (error && !profile) {
    return (
      <section className="card">
        <p className="error">{error}</p>
      </section>
    );
  }
  if (!profile) return <p className="muted center">Loading…</p>;

  const { user, can_view: canView } = profile;
  const isMine = profile.follow_status === "self";

  return (
    <>
      <section className="card profile-header">
        <Avatar user={user} size={96} />
        <div className="profile-title">
          <h1>
            {user.first_name} {user.last_name}
          </h1>
          {user.nickname && <p className="muted">@{user.nickname}</p>}
          {canView && (
            <p className="profile-counts">
              <strong>{profile.follower_count}</strong> followers · <strong>{profile.following_count}</strong> following
              <span className={`pill ${user.is_public ? "pill-public" : "pill-private"}`}>
                {user.is_public ? "Public" : "Private"}
              </span>
            </p>
          )}
        </div>

        {isMine && (
          <label className="switch" title="A private profile is only visible to your followers">
            <input type="checkbox" checked={!user.is_public} onChange={togglePrivacy} disabled={saving} />
            <span className="switch-track" aria-hidden="true" />
            <span>Private profile</span>
          </label>
        )}
      </section>

      {error && <p className="form-error">{error}</p>}

      {!canView ? (
        <section className="card private-notice">
          <span className="lock" aria-hidden="true">
            🔒
          </span>
          <h2>This profile is private</h2>
          <p className="muted">Only followers of {user.first_name} can see their details and posts.</p>
        </section>
      ) : (
        <>
          <section className="card">
            <h2>About</h2>
            <p className={user.about_me ? "about" : "muted"}>{user.about_me || "Nothing here yet."}</p>
            <dl className="details">
              <dt>Email</dt>
              <dd>{user.email}</dd>
              <dt>Date of birth</dt>
              <dd>{user.date_of_birth}</dd>
              <dt>Joined</dt>
              <dd>{user.created_at.slice(0, 10)}</dd>
            </dl>
          </section>

          <section className="card">
            <div className="tabs" role="tablist">
              {TABS.map((t) => (
                <button key={t} role="tab" aria-selected={tab === t} className={tab === t ? "active" : ""} onClick={() => setTab(t)}>
                  {t}
                </button>
              ))}
            </div>
            <p className="muted tab-placeholder">{tab} will be listed here in the next phase.</p>
          </section>
        </>
      )}
    </>
  );
}
