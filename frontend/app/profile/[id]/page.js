"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { parseTime } from "@/lib/time";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import FollowButton from "@/components/FollowButton";
import FollowRequests from "@/components/FollowRequests";
import Icon from "@/components/Icon";
import MessageButton from "@/components/MessageButton";
import NewPostButton from "@/components/NewPostButton";
import UserList from "@/components/UserList";
import { FOLLOWS_CHANGED } from "@/components/NotificationBell";
import { useRealtime } from "@/components/RealtimeProvider";
import PostList from "@/components/PostList";
import { Loading } from "@/components/RacingLights";
import usePagedPosts from "@/lib/usePagedPosts";

const TABS = ["Posts", "Followers", "Following"];

function formatBirthday(date) {
  const [y, m, d] = date.split("-").map(Number);
  return new Date(y, m - 1, d).toLocaleDateString(undefined, { year: "numeric", month: "long", day: "numeric" });
}

function monthYear(time) {
  return parseTime(time).toLocaleDateString(undefined, { month: "short", year: "numeric" });
}

function AboutRow({ icon, label, children, tone }) {
  return (
    <li className={`about-row${tone ? ` about-row-${tone}` : ""}`}>
      <span className="about-row-icon">
        <Icon name={icon} size={18} />
      </span>
      <span className="about-row-text">
        <span className="about-row-label">{label}</span>
        <span className="about-row-value">{children}</span>
      </span>
    </li>
  );
}

export default function ProfilePage() {
  const params = useParams();
  const { user: me, setUser } = useAuth();
  const id = params.id === "me" ? String(me.id) : params.id;

  const [profile, setProfile] = useState(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState(TABS[0]);
  const [saving, setSaving] = useState(false);
  const [list, setList] = useState(null);
  const [listError, setListError] = useState("");

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
    setTab(TABS[0]);
    load();
  }, [load]);

  const canView = profile?.can_view;
  const posts = usePagedPosts(canView && tab === "Posts" ? `/api/users/${encodeURIComponent(id)}/posts` : null);
  useRealtime((event) => {
    if (event.type === "post_created" && !event.payload.group_id && String(event.payload.author_id) === String(id)) posts.refreshTop();
  });
  const listKind = tab === "Followers" ? "followers" : tab === "Following" ? "following" : null;

  useEffect(() => {
    setList(null);
    setListError("");
    if (!listKind || !canView) return;
    api(`/api/users/${encodeURIComponent(id)}/${listKind}`)
      .then(setList)
      .catch((err) => setListError(err.status === 403 ? "This list is private." : err.message));
  }, [id, listKind, canView, profile]);

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
    const isPublic = !e.target.checked;
    setSaving(true);
    setError("");
    try {
      const updated = await api("/api/me/privacy", { method: "PATCH", body: { is_public: isPublic } });
      setUser(updated);
      await load();
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
  if (!profile) return <Loading />;

  const { user } = profile;
  const isMine = profile.follow_status === "self";

  return (
    <div className="profile-page">
      <header className="profile-hero">
        <Avatar user={user} size={112} />
        <div className="profile-title">
          <h1>
            {user.first_name} {user.last_name}
          </h1>
          <p className="profile-sub">
            {user.nickname && <span>@{user.nickname}</span>}
            {user.nickname && user.created_at && <span aria-hidden="true"> · </span>}
            {user.created_at && <span>Joined {monthYear(user.created_at)}</span>}
          </p>
          {canView && (
            <p className="profile-counts">
              <button className="link-btn" onClick={() => setTab("Followers")}>
                <strong>{profile.follower_count}</strong> Followers
              </button>
              <button className="link-btn" onClick={() => setTab("Following")}>
                <strong>{profile.following_count}</strong> Following
              </button>
            </p>
          )}
        </div>

        {isMine ? (
          <div className="profile-actions">
            <label className="privacy-card" title="A private profile is only visible to your followers">
              <span className="privacy-card-text">
                <span className="privacy-card-label">Privacy</span>
                <span className="privacy-card-value">{user.is_public ? "Public" : "Private"}</span>
              </span>
              <span className="switch">
                <input type="checkbox" checked={!user.is_public} onChange={togglePrivacy} disabled={saving} aria-label="Private profile" />
                <span className="switch-track" aria-hidden="true" />
              </span>
            </label>
            <Link href="/settings" className="btn-secondary">
              <Icon name="settings" size={17} />
              Edit Profile
            </Link>
          </div>
        ) : (
          <div className="profile-actions">
            <FollowButton userId={user.id} status={profile.follow_status} onChange={load} />
            <MessageButton user={user} reloadKey={profile} />
          </div>
        )}
      </header>

      {canView && (
        <div className="tabs profile-tabs" role="tablist">
          {TABS.map((t) => (
            <button key={t} role="tab" aria-selected={tab === t} className={tab === t ? "active" : ""} onClick={() => setTab(t)}>
              {t}
              {t === "Followers" && <span className="count-badge">{profile.follower_count}</span>}
              {t === "Following" && <span className="count-badge">{profile.following_count}</span>}
            </button>
          ))}
        </div>
      )}

      {error && <p className="form-error">{error}</p>}

      {!canView ? (
        <section className="card private-notice">
          <span className="lock" aria-hidden="true">
            <Icon name="lock" size={30} />
          </span>
          <h2>This profile is private</h2>
          <p className="muted">
            {profile.follow_status === "pending"
              ? `Your follow request is waiting for ${user.first_name} to accept it.`
              : `Follow ${user.first_name} to see their details, posts and followers.`}
          </p>
        </section>
      ) : (
        <div className="profile-body">
          <div className="profile-main">
            {tab === "Posts" && (
              <PostList
                feed={posts}
                empty={isMine ? "You have not posted anything yet." : `${user.first_name} has no posts you can see.`}
              />
            )}
            {listKind && listError && <p className="form-error tab-placeholder">{listError}</p>}
            {listKind && !list && !listError && <Loading className="muted tab-placeholder" />}
            {listKind && list && (
              <section className="card">
                <UserList
                  users={list}
                  meId={me.id}
                  empty={
                    listKind === "followers"
                      ? `${isMine ? "You have" : `${user.first_name} has`} no followers yet.`
                      : `${isMine ? "You are" : `${user.first_name} is`} not following anyone yet.`
                  }
                />
              </section>
            )}
          </div>

          <aside className="profile-side">
            {isMine && <FollowRequests />}
            <h2 className="side-heading">About</h2>
            {user.about_me && <p className="about about-bio">{user.about_me}</p>}
            <ul className="about-list">
              <AboutRow icon={user.is_public ? "globe" : "lock"} label="Privacy">
                {user.is_public ? "Public" : "Private"}
              </AboutRow>
              {user.email && (
                <AboutRow icon="mail" label="Email">
                  {user.email}
                </AboutRow>
              )}
              {user.date_of_birth && (
                <AboutRow icon="cake" label="Birthday">
                  {formatBirthday(user.date_of_birth)}
                </AboutRow>
              )}
              {user.created_at && (
                <AboutRow icon="shield" label="Member since">
                  {monthYear(user.created_at)}
                </AboutRow>
              )}
              {isMine &&
                (user.is_verified ? (
                  <AboutRow icon="check" label="Verified" tone="ok">
                    Email address confirmed
                  </AboutRow>
                ) : (
                  <li className="about-row about-row-alert">
                    <Link href="/verify" className="about-row-link">
                      <span className="about-row-icon">
                        <Icon name="shieldAlert" size={18} />
                      </span>
                      <span className="about-row-text">
                        <span className="about-row-label">Unverified</span>
                        <span className="about-row-value">Click to verify account</span>
                      </span>
                    </Link>
                  </li>
                ))}
            </ul>
          </aside>
        </div>
      )}

      {isMine && canView && tab === "Posts" && <NewPostButton onCreated={posts.prepend} />}
    </div>
  );
}
