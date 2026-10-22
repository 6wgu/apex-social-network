"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { parseTime, timeAgo } from "@/lib/time";
import Avatar from "@/components/Avatar";
import Icon from "@/components/Icon";
import ImagePicker from "@/components/ImagePicker";
import { notify } from "@/components/Toaster";
import { PRIVACY } from "@/components/PostComposer";
import { useRealtime } from "@/components/RealtimeProvider";
import { useAuth } from "@/components/AuthProvider";

const COMMENTS_PAGE = 50; // the backend's page size for comments

function Author({ user, time, privacy }) {
  return (
    <div className="post-author">
      <Link href={`/profile/${user.id}`}>
        <Avatar user={user} size={40} />
      </Link>
      <div>
        <Link href={`/profile/${user.id}`} className="post-name">
          {user.first_name} {user.last_name}
        </Link>
        <span className="post-meta">
          <time className="post-time" dateTime={parseTime(time).toISOString()} title={parseTime(time).toLocaleString()}>
            {timeAgo(time)}
          </time>
          {privacy && (
            <span className="post-privacy" title={`${privacy.label}: ${privacy.hint}`}>
              <span aria-hidden="true">·</span>
              <Icon name={privacy.icon} size={13} />
              {privacy.label}
            </span>
          )}
        </span>
      </div>
    </div>
  );
}

// The comments under a post: loaded when opened, oldest first, with a form
// to add one (text and/or image). The backend answers 404 for a post the
// user may not see, so there is no client-side permission logic here.
function Comments({ postId, postAuthorId, onAdded, onRemoved }) {
  const { user } = useAuth();
  const [comments, setComments] = useState(null);
  const [hasMore, setHasMore] = useState(false);
  const [error, setError] = useState("");
  const [content, setContent] = useState("");
  const [image, setImage] = useState(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(
    async (after) => {
      try {
        const page = await api(`/api/posts/${postId}/comments${after ? `?after=${after}` : ""}`);
        setComments((old) => (after && old ? [...old, ...page] : page));
        setHasMore(page.length === COMMENTS_PAGE);
      } catch (err) {
        setError(err.message);
      }
    },
    [postId]
  );

  useEffect(() => {
    load();
  }, [load]);

  // Adds a comment once, whether it came from our own form or live.
  const append = (c) => setComments((old) => (old && old.some((x) => x.id === c.id) ? old : [...(old || []), c]));
  // Drops a comment once, whether we deleted it or someone else did.
  const commentsRef = useRef(comments);
  commentsRef.current = comments;
  const drop = (commentId) => {
    if (!commentsRef.current?.some((x) => x.id === commentId)) return;
    commentsRef.current = commentsRef.current.filter((x) => x.id !== commentId);
    setComments(commentsRef.current);
    onRemoved(commentId);
  };
  useRealtime((event) => {
    if (event.type === "comment_created" && event.payload.post_id === postId) append(event.payload);
    if (event.type === "comment_deleted" && event.payload.post_id === postId) drop(event.payload.comment_id);
  });

  async function remove(c) {
    if (!window.confirm("Delete this comment?")) return;
    try {
      await api(`/api/posts/${postId}/comments/${c.id}`, { method: "DELETE" });
      drop(c.id);
    } catch (err) {
      notify({ tone: "error", title: "Couldn't delete the comment", message: err.message });
    }
  }

  async function submit(e) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const form = new FormData();
    form.append("content", content);
    if (image) form.append("image", image);
    try {
      const c = await api(`/api/posts/${postId}/comments`, { method: "POST", body: form });
      append(c);
      setContent("");
      setImage(null);
      onAdded(c);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="comments">
      {comments === null && !error && <p className="muted">Loading comments…</p>}
      {comments && comments.length === 0 && <p className="muted">No comments yet. Be the first!</p>}
      <ul>
        {(comments || []).map((c) => (
          <li key={c.id} className="comment">
            <Link href={`/profile/${c.author.id}`}>
              <Avatar user={c.author} size={32} />
            </Link>
            <div className="comment-bubble">
              <Link href={`/profile/${c.author.id}`} className="post-name">
                {c.author.first_name} {c.author.last_name}
              </Link>{" "}
              <span className="muted post-time">{timeAgo(c.created_at)}</span>
              {user && (user.id === c.author.id || user.id === postAuthorId) && (
                <button type="button" className="delete-btn" onClick={() => remove(c)} aria-label="Delete comment" title="Delete comment">
                  <Icon name="trash" size={15} />
                </button>
              )}
              {c.content && <p className="post-text">{c.content}</p>}
              {c.image_path && (
                // eslint-disable-next-line @next/next/no-img-element
                <img className="comment-image" src={c.image_path} alt="Comment image" loading="lazy" />
              )}
            </div>
          </li>
        ))}
      </ul>
      {hasMore && (
        <button className="link-btn more-comments" onClick={() => load(comments[comments.length - 1].id)}>
          Show more comments
        </button>
      )}
      <form className="comment-form" onSubmit={submit}>
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="Write a comment…"
          rows={1}
          maxLength={2000}
          aria-label="Comment text"
        />
        <ImagePicker value={image} onChange={setImage} compact label="Add an image to the comment" />
        <button className="btn-primary" disabled={busy || (!content.trim() && !image)}>
          Send
        </button>
      </form>
      {error && <p className="form-error">{error}</p>}
    </div>
  );
}

// The ♥ button: updates at once, then settles on the server's answer
// (or goes back if the request fails).
function LikeButton({ post }) {
  const [liked, setLiked] = useState(Boolean(post.liked_by_me));
  const [count, setCount] = useState(post.like_count || 0);
  const [busy, setBusy] = useState(false);

  // Someone else liked or unliked: take the new count (our own click
  // settles on the server's answer instead).
  useRealtime((event) => {
    if (event.type === "post_likes" && event.payload.post_id === post.id && !busy) setCount(event.payload.like_count);
  });

  async function toggle() {
    if (busy) return;
    const next = !liked;
    setBusy(true);
    setLiked(next);
    setCount((c) => Math.max(0, c + (next ? 1 : -1)));
    try {
      const s = await api(`/api/posts/${post.id}/like`, { method: next ? "POST" : "DELETE" });
      setLiked(s.liked_by_me);
      setCount(s.like_count);
    } catch (err) {
      setLiked(!next);
      setCount((c) => Math.max(0, c + (next ? -1 : 1)));
      notify({ tone: "error", title: "Couldn't update your like", message: err.message });
    } finally {
      setBusy(false);
    }
  }

  return (
    <button
      className={`post-action post-like${liked ? " is-liked" : ""}`}
      onClick={toggle}
      aria-pressed={liked}
      aria-label={`${liked ? "Unlike" : "Like"}, ${count} ${count === 1 ? "like" : "likes"}`}
    >
      <Icon name="heart" size={20} />
      <span>{count}</span>
    </button>
  );
}

// One post: author, time, privacy icon, text, image, likes and comments.
// A group post has no privacy icon: its group decides who sees it.
export default function PostCard({ post }) {
  const [open, setOpen] = useState(false);
  const [count, setCount] = useState(post.comment_count);
  // Each new comment counts once, from the form or from the live push.
  const seen = useRef(new Set());
  const counted = (c) => {
    if (seen.current.has(c.id)) return;
    seen.current.add(c.id);
    setCount((n) => n + 1);
  };
  useRealtime((event) => {
    if (event.type === "comment_created" && event.payload.post_id === post.id) counted(event.payload);
  });
  const privacy = post.group_id ? null : PRIVACY[post.privacy];
  const { user } = useAuth();
  const [gone, setGone] = useState(false);
  useRealtime((event) => {
    if (event.type === "post_deleted" && event.payload.post_id === post.id) setGone(true);
    if (event.type === "comment_deleted" && event.payload.post_id === post.id && !open) setCount((n) => Math.max(0, n - 1));
  });

  async function removePost() {
    if (!window.confirm("Delete this post? Its comments and likes will be deleted too.")) return;
    try {
      await api(`/api/posts/${post.id}`, { method: "DELETE" });
      setGone(true);
      notify({ message: "Post deleted" });
    } catch (err) {
      notify({ tone: "error", title: "Couldn't delete the post", message: err.message });
    }
  }

  if (gone) return null;

  return (
    <article className="card post">
      <header className="post-head">
        <Author user={post.author} time={post.created_at} privacy={privacy} />
        {user?.id === post.author.id && (
          <button type="button" className="delete-btn" onClick={removePost} aria-label="Delete post" title="Delete post">
            <Icon name="trash" size={18} />
          </button>
        )}
      </header>
      {post.content && <p className="post-text">{post.content}</p>}
      {post.image_path && (
        // eslint-disable-next-line @next/next/no-img-element
        <img className="post-image" src={post.image_path} alt="Post image" loading="lazy" />
      )}
      <footer className="post-foot">
        <LikeButton post={post} />
        <button className="post-action" onClick={() => setOpen(!open)} aria-expanded={open} aria-label={`${count} ${count === 1 ? "comment" : "comments"}`}>
          <Icon name="comment" size={20} />
          <span>{count}</span>
        </button>
      </footer>
      {open && (
        <Comments
          postId={post.id}
          postAuthorId={post.author.id}
          onAdded={counted}
          onRemoved={() => setCount((n) => Math.max(0, n - 1))}
        />
      )}
    </article>
  );
}
