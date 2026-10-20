"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import usePagedPosts from "@/lib/usePagedPosts";
import useAutoRefresh from "@/lib/useAutoRefresh";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import Icon from "@/components/Icon";
import Landing from "@/components/Landing";
import NewPostButton from "@/components/NewPostButton";
import PostCard from "@/components/PostCard";
import { Loading } from "@/components/RacingLights";
import { notify } from "@/components/Toaster";
import { useRealtime } from "@/components/RealtimeProvider";
import WebmasterPost, { isWebmasterPost } from "@/components/WebmasterPost";

// The feed as a column of cards that snap into place, with up / down
// buttons beside it. More posts load near the end.
function FeedReel({ feed }) {
  const { posts, hasMore, loading, error, loadMore } = feed;
  const box = useRef(null);
  const frame = useRef(0);
  const [index, setIndex] = useState(0);

  const items = () => [...(box.current?.querySelectorAll(":scope > .post") || [])];

  const onScroll = useCallback(() => {
    cancelAnimationFrame(frame.current);
    frame.current = requestAnimationFrame(() => {
      const el = box.current;
      if (!el) return;
      const list = items();
      const i = list.findIndex((it) => it.offsetTop + it.offsetHeight - 80 > el.scrollTop);
      setIndex(Math.max(0, i));
      if (hasMore && !loading && el.scrollHeight - el.scrollTop - el.clientHeight < el.clientHeight * 1.5) loadMore();
    });
  }, [hasMore, loading, loadMore]);

  useEffect(() => () => cancelAnimationFrame(frame.current), []);

  function go(step) {
    const list = items();
    const target = list[Math.max(0, Math.min(list.length - 1, index + step))];
    const calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    target?.scrollIntoView({ behavior: calm ? "auto" : "smooth", block: "start" });
  }

  // The webmaster's post always comes first: the saved one when the
  // database has it (likes and comments work), the hard-coded copy if not.
  const saved = posts?.find(isWebmasterPost);
  const count = (posts?.filter((p) => !isWebmasterPost(p)).length || 0) + 1; // + the webmaster's post

  return (
    <>
      <div className="feed-reel" ref={box} onScroll={onScroll}>
        {saved ? <PostCard key={saved.id} post={saved} /> : <WebmasterPost inFeed />}
        {posts && posts.filter((p) => !isWebmasterPost(p)).map((p) => <PostCard key={p.id} post={p} />)}
        {posts && posts.length === 0 && !loading && (
          <div className="card feed-empty">
            <h2>Your feed is quiet</h2>
            <p className="muted">No posts yet. Write the first one, or follow people to see theirs.</p>
          </div>
        )}
        {error && <p className="form-error">{error}</p>}
        {loading && <Loading>Loading posts…</Loading>}
        {hasMore && !loading && (
          <button className="btn-secondary load-more" onClick={loadMore}>
            Load more
          </button>
        )}
      </div>
      {count > 1 && (
        <div className="reel-arrows">
          <button type="button" className="reel-arrow" onClick={() => go(-1)} disabled={index === 0} aria-label="Previous post">
            <Icon name="arrowUp" size={18} />
          </button>
          <button type="button" className="reel-arrow" onClick={() => go(1)} disabled={index >= count - 1} aria-label="Next post">
            <Icon name="arrowDown" size={18} />
          </button>
        </div>
      )}
    </>
  );
}

// Right column: people you can chat with, and groups you have not joined.
function HomeSide() {
  const [friends, setFriends] = useState(null);
  const [groups, setGroups] = useState(null);

  useEffect(() => {
    api("/api/chat-contacts").then(setFriends).catch(() => setFriends([]));
    api("/api/groups")
      .then((list) => setGroups(list.filter((g) => g.my_status === "none").slice(0, 5)))
      .catch(() => setGroups([]));
  }, []);

  return (
    <aside className="home-side">
      <section className="side-section">
        <h2 className="side-heading">Friends</h2>
        {!friends && <Loading className="muted" />}
        {friends && friends.length === 0 && <p className="side-empty">Follow people to see them here.</p>}
        <ul className="side-list">
          {(friends || []).slice(0, 8).map((u) => (
            <li key={u.id}>
              <Link href={`/profile/${u.id}`} className="side-row">
                <Avatar user={u} size={36} />
                <span className="side-row-name">
                  <span className="side-row-title">
                    {u.first_name} {u.last_name}
                  </span>
                </span>
              </Link>
              <Link href={`/chats?user=${u.id}`} className="side-row-action" aria-label={`Message ${u.first_name}`}>
                <Icon name="chat" size={17} />
              </Link>
            </li>
          ))}
        </ul>
      </section>

      <section className="side-section">
        <div className="side-heading-row">
          <h2 className="side-heading">Suggested groups</h2>
          <Link href="/groups" className="side-link">
            See all
          </Link>
        </div>
        {!groups && <Loading className="muted" />}
        {groups && groups.length === 0 && <p className="side-empty">You are in every group.</p>}
        <ul className="side-list">
          {(groups || []).map((g) => (
            <li key={g.id}>
              <Link href={`/groups/${g.id}`} className="side-row">
                <span className="side-group-icon" aria-hidden="true">
                  {g.title[0].toUpperCase()}
                </span>
                <span className="side-row-name">
                  <span className="side-row-title">{g.title}</span>
                  <span className="side-row-meta">
                    {g.member_count} {g.member_count === 1 ? "member" : "members"}
                  </span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </aside>
  );
}

// The signed-in home: the feed (every post you may see, newest first), with
// the New Post button, and the side column.
function Feed() {
  const { user } = useAuth();
  const feed = usePagedPosts("/api/feed");
  const checkNew = async (quiet) => {
    const added = await feed.refreshTop();
    if (added && !quiet) notify({ message: `${added} new ${added === 1 ? "post" : "posts"} in your feed` });
  };
  // New posts arrive live; the timer catches any the socket missed.
  useRealtime((event) => {
    if (event.type === "post_created" && !event.payload.group_id) checkNew(event.payload.author_id === user?.id);
  });
  useAutoRefresh(() => checkNew(false), 20000);

  return (
    <div className="home-grid">
      <div className="home-main">
        <FeedReel feed={feed} />
        <NewPostButton onCreated={feed.prepend} />
      </div>
      <HomeSide />
    </div>
  );
}

// Home: the landing page when logged out, the feed when logged in.
export default function Home() {
  const { user } = useAuth();
  return user ? <Feed /> : <Landing />;
}
