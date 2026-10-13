"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { parseTime, timeAgo } from "@/lib/time";
import Avatar from "@/components/Avatar";
import ImagePicker from "@/components/ImagePicker";
import { PRIVACY } from "@/components/PostComposer";

const COMMENTS_PAGE = 50; // the backend's page size for comments

function Author({ user, time }) {
  return (
    <div className="post-author">
      <Link href={`/profile/${user.id}`}>
        <Avatar user={user} size={40} />
      </Link>
      <div>
        <Link href={`/profile/${user.id}`} className="post-name">
          {user.first_name} {user.last_name}
        </Link>
        <time className="muted post-time" dateTime={parseTime(time).toISOString()} title={parseTime(time).toLocaleString()}>
          {timeAgo(time)}
        </time>
      </div>
    </div>
  );
}

// The comments under a post: loaded when opened, oldest first, with a form
// to add one (text and/or image). The backend answers 404 for a post the
// user may not see, so there is no client-side permission logic here.
function Comments({ postId, onAdded }) {
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

  async function submit(e) {
    e.preventDefault();
    setError("");
    setBusy(true);
    const form = new FormData();
    form.append("content", content);
    if (image) form.append("image", image);
    try {
      const c = await api(`/api/posts/${postId}/comments`, { method: "POST", body: form });
      setComments((old) => [...(old || []), c]);
      setContent("");
      setImage(null);
      onAdded();
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

// One post: author, time, privacy icon, text, image, and its comments.
export default function PostCard({ post }) {
  const [open, setOpen] = useState(false);
  const [count, setCount] = useState(post.comment_count);
  const privacy = PRIVACY[post.privacy];

  return (
    <article className="card post">
      <header className="post-head">
        <Author user={post.author} time={post.created_at} />
        <span className="post-privacy" title={`${privacy.label}: ${privacy.hint}`} aria-label={privacy.label}>
          {privacy.icon}
        </span>
      </header>
      {post.content && <p className="post-text">{post.content}</p>}
      {post.image_path && (
        // eslint-disable-next-line @next/next/no-img-element
        <img className="post-image" src={post.image_path} alt="Post image" loading="lazy" />
      )}
      <footer className="post-foot">
        <button className="link-btn" onClick={() => setOpen(!open)} aria-expanded={open}>
          💬 {count} {count === 1 ? "comment" : "comments"}
        </button>
      </footer>
      {open && <Comments postId={post.id} onAdded={() => setCount((c) => c + 1)} />}
    </article>
  );
}
