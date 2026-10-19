"use client";

import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import usePagedPosts from "@/lib/usePagedPosts";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import ChatWindow from "@/components/ChatWindow";
import GroupEvents from "@/components/GroupEvents";
import GroupJoinButton from "@/components/GroupJoinButton";
import InviteDialog from "@/components/InviteDialog";
import JoinRequests from "@/components/JoinRequests";
import { GROUPS_CHANGED } from "@/components/NotificationBell";
import PostComposer from "@/components/PostComposer";
import PostList from "@/components/PostList";
import { Loading } from "@/components/RacingLights";
import { useRealtime, useRealtimeSend } from "@/components/RealtimeProvider";
import UserList from "@/components/UserList";

// /groups/{id}, optionally ?tab=Events (links from the bell open a tab).
// Everyone sees the header; posts, events and members are only requested
// for members, and the backend answers 403 to anyone else anyway.
export default function GroupPage() {
  const { id } = useParams();
  const searchParams = useSearchParams();
  const { user: me } = useAuth();
  const send = useRealtimeSend();
  const [group, setGroup] = useState(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState("Posts");
  const [inviting, setInviting] = useState(false);
  const [reloadKey, setReloadKey] = useState(0); // bumps when events or requests may have changed

  const load = useCallback(
    () =>
      api(`/api/groups/${encodeURIComponent(id)}`)
        .then((g) => {
          setGroup(g);
          setError("");
        })
        .catch((err) => setError(err.status === 404 ? "This group does not exist." : err.message)),
    [id]
  );

  useEffect(() => {
    setGroup(null);
    setError("");
    setInviting(false);
    load();
  }, [load]);

  const askedTab = searchParams.get("tab");
  useEffect(() => {
    setTab(askedTab || "Posts");
  }, [askedTab, id]);

  // Answers given in the bell, and live notifications about this group
  // (a new event, a join request, being accepted), reload what they touch.
  useEffect(() => {
    window.addEventListener(GROUPS_CHANGED, load);
    return () => window.removeEventListener(GROUPS_CHANGED, load);
  }, [load]);
  useRealtime((event) => {
    // Someone joined: reload the group (member list and count).
    if (event.type === "group_changed") {
      if (String(event.payload.group_id) === String(id)) load();
      return;
    }
    if (event.type !== "notification" || String(event.payload.group?.id) !== String(id)) return;
    load();
    setReloadKey((k) => k + 1);
  });

  const isMember = group?.my_status === "member";
  const isCreator = group && group.creator.id === me.id;
  const posts = usePagedPosts(isMember && tab === "Posts" ? `/api/groups/${encodeURIComponent(id)}/posts` : null);
  // A new post in this group shows up at once.
  useRealtime((event) => {
    if (event.type === "post_created" && String(event.payload.group_id) === String(id)) posts.refreshTop();
  });

  if (error && !group) {
    return (
      <section className="card">
        <p className="error">{error}</p>
        <Link href="/groups">← All groups</Link>
      </section>
    );
  }
  if (!group) return <Loading />;

  const tabs = ["Posts", "Chat", "Events", "Members", ...(isCreator ? ["Requests"] : [])];
  const shownTab = tabs.includes(tab) ? tab : "Posts";

  return (
    <>
      <section className="card group-header">
        <div className="group-icon group-icon-large" aria-hidden="true">
          {group.title[0].toUpperCase()}
        </div>
        <div className="group-heading">
          <h1>{group.title}</h1>
          <p className="muted">
            {group.member_count} {group.member_count === 1 ? "member" : "members"} · created by{" "}
            <Link href={`/profile/${group.creator.id}`}>
              {group.creator.first_name} {group.creator.last_name}
            </Link>
            {isMember && <span className="pill pill-public">{isCreator ? "Creator" : "Member"}</span>}
          </p>
          <p className="about group-about">{group.description}</p>
        </div>
        <div className="profile-actions">
          {isMember ? (
            <button className="btn-primary" onClick={() => setInviting(true)}>
              + Invite people
            </button>
          ) : (
            <GroupJoinButton group={group} onChange={load} />
          )}
        </div>
      </section>

      {!isMember ? (
        <section className="card private-notice">
          <span className="lock" aria-hidden="true">
            🔒
          </span>
          <h2>Only members can see inside this group</h2>
          <p className="muted">
            {group.my_status === "requested"
              ? `Your request is waiting for ${group.creator.first_name} to accept it.`
              : group.my_status === "invited"
                ? "You have been invited: accept the invitation to see the posts, events and members."
                : `Ask to join, or ask a member to invite you. ${group.creator.first_name} answers join requests.`}
          </p>
        </section>
      ) : (
        <section className="card">
          <div className="tabs" role="tablist">
            {tabs.map((t) => (
              <button
                key={t}
                role="tab"
                aria-selected={shownTab === t}
                className={shownTab === t ? "active" : ""}
                onClick={() => setTab(t)}
              >
                {t}
              </button>
            ))}
          </div>
          {shownTab === "Posts" && (
            <>
              <PostComposer groupId={group.id} onCreated={posts.prepend} />
              <PostList feed={posts} empty="No posts in this group yet. Start the conversation!" />
            </>
          )}
          {shownTab === "Chat" && (
            <ChatWindow
              historyUrl={`/api/groups/${group.id}/messages`}
              matches={(e) => e.type === "group_message" && e.payload.group_id === group.id}
              send={(content) => send("group_message", { group_id: group.id, content })}
              showNames
            />
          )}
          {shownTab === "Events" && <GroupEvents groupId={group.id} reloadKey={reloadKey} />}
          {shownTab === "Members" && (
            <>
              <UserList users={group.members} meId={me.id} empty="No members." />
              {group.invited_users.length > 0 && (
                <>
                  <h3 className="list-heading">Invited, waiting for an answer</h3>
                  <ul className="people">
                    {group.invited_users.map((u) => (
                      <li key={u.id}>
                        <Link href={`/profile/${u.id}`}>
                          <Avatar user={u} size={40} />
                          <span>
                            {u.first_name} {u.last_name}
                          </span>
                        </Link>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </>
          )}
          {shownTab === "Requests" && isCreator && <JoinRequests groupId={group.id} reloadKey={reloadKey} onAnswered={load} />}
        </section>
      )}

      {inviting && <InviteDialog group={group} me={me} onClose={() => setInviting(false)} onInvited={load} />}
    </>
  );
}
