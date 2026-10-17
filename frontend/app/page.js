"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import usePagedPosts from "@/lib/usePagedPosts";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import PostComposer from "@/components/PostComposer";
import PostList from "@/components/PostList";

function LandingPage() {
  return (
    <div className="landing">
      <section className="landing-hero">
        <div className="landing-copy">
          <p className="eyebrow">A quieter way to stay close</p>
          <h1>Share the moments that matter.</h1>
          <p className="landing-lede">
            A private, people-first social space for your updates, conversations, groups, and the people you actually want to hear from.
          </p>
          <div className="landing-actions">
            <Link href="/register" className="btn-primary btn-large">Create your account</Link>
            <Link href="/login" className="text-link">Already a member? Log in</Link>
          </div>
        </div>
        <div className="landing-preview" aria-label="Social network preview">
          <div className="preview-topline"><span className="preview-dot" /><span>your circle</span><span className="preview-status">private by default</span></div>
          <div className="preview-post preview-post-featured">
            <div className="preview-avatar avatar-initials">AM</div>
            <div><strong>Amira Malik</strong><span> · 8 min ago</span><p>Sunday light, good coffee, and nowhere else to be.</p></div>
          </div>
          <div className="preview-stats"><span><b>24</b> close friends</span><span><b>06</b> active groups</span></div>
          <div className="preview-post preview-post-secondary"><div className="preview-avatar preview-avatar-gold">JT</div><div><strong>Jonas Taylor</strong><span> · now</span><p>Anyone up for a walk?</p></div><span className="preview-reply">Reply</span></div>
        </div>
      </section>
      <section className="landing-pillars" aria-label="Product highlights">
        <article><span className="pillar-number">01</span><h2>Keep it personal</h2><p>Choose who sees your profile, posts, and conversations.</p></article>
        <article><span className="pillar-number">02</span><h2>Find your people</h2><p>Follow friends, discover groups, and build smaller circles.</p></article>
        <article><span className="pillar-number">03</span><h2>Stay in the moment</h2><p>Chat live, share updates, and get notifications that feel useful.</p></article>
      </section>
    </div>
  );
}

// People search (GET /api/users?search=), shown beside the feed.
function FindPeople({ me }) {
  const [search, setSearch] = useState("");
  const [people, setPeople] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    // Wait for a pause in typing before asking the server.
    const timer = setTimeout(() => {
      api(`/api/users?search=${encodeURIComponent(search)}`)
        .then((list) => {
          setPeople(list);
          setError("");
        })
        .catch((err) => setError(err.message));
    }, 250);
    return () => clearTimeout(timer);
  }, [search]);

  return (
    <section className="card">
      <h2>Find people</h2>
      <input
        type="search"
        className="search"
        placeholder="Search by name or nickname"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        aria-label="Search people"
      />
      {error && <p className="form-error">{error}</p>}
      {people && people.length === 0 && <p className="muted">Nobody matches that search.</p>}
      <ul className="people">
        {(people || []).map((p) => (
          <li key={p.id}>
            <Link href={`/profile/${p.id}`}>
              <Avatar user={p} size={36} />
              <span>
                <strong>
                  {p.first_name} {p.last_name}
                </strong>
                {p.nickname && <span className="muted"> @{p.nickname}</span>}
                {p.id === me.id && <span className="muted"> (you)</span>}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}

// Home: write a post, then the feed (every post you may see, newest first).
export default function Home() {
  const { user } = useAuth();

  if (!user) return <LandingPage />;

  const feed = usePagedPosts("/api/feed");

  return (
    <div className="home-grid">
      <div className="home-main">
        <PostComposer onCreated={feed.prepend} />
        <PostList feed={feed} empty="No posts yet. Write the first one, or follow people to see theirs." />
      </div>
      <aside className="home-side">
        <FindPeople me={user} />
      </aside>
    </div>
  );
}
