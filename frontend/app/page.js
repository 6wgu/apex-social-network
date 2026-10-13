"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";

// Phase 1 home page: a welcome and a people search (GET /api/users?search=).
// The feed replaces the welcome card in Phase 2.
export default function Home() {
  const { user } = useAuth();
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
    <>
      <section className="card">
        <h1>Welcome, {user.first_name}</h1>
        <p className="muted">
          Posts from people you follow will show up here. For now you can find people and visit{" "}
          <Link href={`/profile/${user.id}`}>your profile</Link>.
        </p>
      </section>

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
                <Avatar user={p} size={40} />
                <span>
                  <strong>
                    {p.first_name} {p.last_name}
                  </strong>
                  {p.nickname && <span className="muted"> @{p.nickname}</span>}
                  {p.id === user.id && <span className="muted"> (you)</span>}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </>
  );
}
