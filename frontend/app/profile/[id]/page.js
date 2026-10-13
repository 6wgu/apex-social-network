"use client";

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import EditProfile from "@/components/EditProfile";
import FollowButton from "@/components/FollowButton";
import UserList from "@/components/UserList";
import { FOLLOWS_CHANGED } from "@/components/NotificationBell";
import { useRealtime } from "@/components/RealtimeProvider";
import PostComposer from "@/components/PostComposer";
import PostList from "@/components/PostList";
import usePagedPosts from "@/lib/usePagedPosts";

const TABS = ["Posts", "Followers", "Following"];

// /profile/{id}, or /profile/me for the logged-in user.
// What a private profile reveals is decided by the backend: for a profile we
// may not see, GET /api/users/{id} simply does not contain the private fields,
// and the followers/following lists answer 403.
export default function ProfilePage() {
  const params = useParams();
  const { user: me, setUser } = useAuth();
  const id = params.id === "me" ? String(me.id) : params.id;

  const [profile, setProfile] = useState(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState(TABS[0]);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState(false);
  const [list, setList] = useState(null); // users in the Followers / Following tab
  const [listError, setListError] = useState("");

  // Loads (or reloads) the profile without blanking the page.
  const load = useCallback(
    () =>
      api(`/api/users/${encodeURIComponent(id)}`)
        .then((p) => {
          setProfile(p);
          setError("");
        })
        .catch((err) => setError(err.status === 404 ? "This user does not exist." : err.message)),
    [id]
  );

  useEffect(() => {
    setProfile(null);
    setError("");
    setEditing(false);
    setTab(TABS[0]);
    load();
  }, [load]);

  const canView = profile?.can_view;
  // The Posts tab; nothing is requested for a profile we may not see.
  const posts = usePagedPosts(canView && tab === "Posts" ? `/api/users/${encodeURIComponent(id)}/posts` : null);
  const listKind = tab === "Followers" ? "followers" : tab === "Following" ? "following" : null;

  // The Followers / Following tab content. Reloads whenever the profile does
  // (e.g. after following), so the list and the counts always agree.
  useEffect(() => {
    setList(null);
    setListError("");
    if (!listKind || !canView) return;
    api(`/api/users/${encodeURIComponent(id)}/${listKind}`)
      .then(setList)
      .catch((err) => setListError(err.status === 403 ? "This list is private." : err.message));
  }, [id, listKind, canView, profile]);

  // Keep counts and visibility current when follows change elsewhere:
  // a request accepted from the bell, or a live notification about this profile.
  useEffect(() => {
    window.addEventListener(FOLLOWS_CHANGED, load);
    return () => window.removeEventListener(FOLLOWS_CHANGED, load);
  }, [load]);
  useRealtime((event) => {
    if (event.type !== "notification" || !profile) return;
    const n = event.payload;
    const aboutThisProfile = n.actor && String(n.actor.id) === String(profile.user.id);
    if ((profile.follow_status === "self" && n.type === "new_follower") || (aboutThisProfile && n.type === "follow_accepted")) {
      load();
    }
  });

  async function togglePrivacy(e) {
    const isPublic = !e.target.checked; // the switch reads "Private profile"
    setSaving(true);
    setError("");
    try {
      const updated = await api("/api/me/privacy", { method: "PATCH", body: { is_public: isPublic } });
      setUser(updated);
      await load(); // going public accepts pending requests, so counts can change
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }

  function saved(updated) {
    setProfile((p) => ({ ...p, user: updated }));
    setUser(updated);
    setEditing(false);
  }

  if (error && !profile) {
    return (
      <section className="card">
        <p className="error">{error}</p>
      </section>
    );
  }
  if (!profile) return <p className="muted center">Loading…</p>;

  const { user } = profile;
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
              <button className="link-btn" onClick={() => setTab("Followers")}>
                <strong>{profile.follower_count}</strong> followers
              </button>{" "}
              ·{" "}
              <button className="link-btn" onClick={() => setTab("Following")}>
                <strong>{profile.following_count}</strong> following
              </button>
              <span className={`pill ${user.is_public ? "pill-public" : "pill-private"}`}>
                {user.is_public ? "Public" : "Private"}
              </span>
            </p>
          )}
        </div>

        {isMine ? (
          <div className="profile-actions">
            <label className="switch" title="A private profile is only visible to your followers">
              <input type="checkbox" checked={!user.is_public} onChange={togglePrivacy} disabled={saving} />
              <span className="switch-track" aria-hidden="true" />
              <span>Private profile</span>
            </label>
            {!editing && (
              <button className="btn-secondary" onClick={() => setEditing(true)}>
                Edit profile
              </button>
            )}
          </div>
        ) : (
          <div className="profile-actions">
            <FollowButton userId={user.id} status={profile.follow_status} onChange={load} />
          </div>
        )}
      </section>

      {error && <p className="form-error">{error}</p>}

      {isMine && editing && <EditProfile user={user} onSaved={saved} onCancel={() => setEditing(false)} />}

      {!canView ? (
        <section className="card private-notice">
          <span className="lock" aria-hidden="true">
            🔒
          </span>
          <h2>This profile is private</h2>
          <p className="muted">
            {profile.follow_status === "pending"
              ? `Your follow request is waiting for ${user.first_name} to accept it.`
              : `Follow ${user.first_name} to see their details, posts and followers.`}
          </p>
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
            {tab === "Posts" && isMine && <PostComposer onCreated={posts.prepend} />}
            {tab === "Posts" && (
              <PostList
                feed={posts}
                empty={isMine ? "You have not posted anything yet." : `${user.first_name} has no posts you can see.`}
              />
            )}
            {listKind && listError && <p className="form-error tab-placeholder">{listError}</p>}
            {listKind && !list && !listError && <p className="muted tab-placeholder">Loading…</p>}
            {listKind && list && (
              <UserList
                users={list}
                meId={me.id}
                empty={
                  listKind === "followers"
                    ? `${isMine ? "You have" : `${user.first_name} has`} no followers yet.`
                    : `${isMine ? "You are" : `${user.first_name} is`} not following anyone yet.`
                }
              />
            )}
          </section>
        </>
      )}
    </>
  );
}
