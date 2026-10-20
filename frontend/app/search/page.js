"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import useSearchQuery from "@/lib/useSearchQuery";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import Icon from "@/components/Icon";
import { Loading } from "@/components/RacingLights";
import SearchInput from "@/components/SearchInput";

// /search: people search (GET /api/users?search=). An empty box lists users,
// loaded once; typing searches once it pauses (useSearchQuery), and only the
// answer for the latest query is shown.
export default function SearchPage() {
  const { user: me } = useAuth();
  const [search, setSearch] = useState("");
  const query = useSearchQuery(search);
  const [everyone, setEveryone] = useState(null);
  const [found, setFound] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api("/api/users")
      .then(setEveryone)
      .catch((err) => setError(err.message));
  }, []);

  useEffect(() => {
    if (!query) return;
    let current = true;
    api(`/api/users?search=${encodeURIComponent(query)}`)
      .then((list) => {
        if (!current) return;
        setFound(list);
        setError("");
      })
      .catch((err) => current && setError(err.message));
    return () => {
      current = false;
    };
  }, [query]);

  const people = query ? found : everyone;

  return (
    <div className="search-page">
      <header className="page-head">
        <h1>Search</h1>
        <p>Find people by name or nickname.</p>
      </header>
      <label className="search-box">
        <Icon name="search" size={19} />
        <SearchInput
          className="search"
          placeholder="Search by name or nickname"
          value={search}
          onChange={setSearch}
          aria-label="Search people"
          autoFocus
        />
      </label>
      {error && <p className="form-error">{error}</p>}
      {!people && !error && <Loading />}
      {people && people.length === 0 && <p className="muted center">Nobody matches that search.</p>}
      <ul className="people people-grid">
        {(people || []).map((p) => (
          <li key={p.id}>
            <Link href={`/profile/${p.id}`}>
              <Avatar user={p} size={44} />
              <span className="people-text">
                <strong>
                  {p.first_name} {p.last_name}
                  {p.id === me.id && <span className="muted"> (you)</span>}
                </strong>
                {p.nickname && <span className="muted">@{p.nickname}</span>}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
