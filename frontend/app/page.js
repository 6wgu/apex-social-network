"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import usePagedPosts from "@/lib/usePagedPosts";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import PostComposer from "@/components/PostComposer";
import PostList from "@/components/PostList";

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
