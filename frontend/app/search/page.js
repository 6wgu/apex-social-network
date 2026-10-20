"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { searchQuery } from "@/lib/search";
import { useAuth } from "@/components/AuthProvider";
import Avatar from "@/components/Avatar";
import Icon from "@/components/Icon";
import { Loading } from "@/components/RacingLights";
import SearchInput from "@/components/SearchInput";

// /search: people search (GET /api/users?search=). An empty search lists users.
export default function SearchPage() {
  const { user: me } = useAuth();
  const [search, setSearch] = useState("");
  const [people, setPeople] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    // Wait for a pause in typing before asking the server.
    const timer = setTimeout(() => {
      api(`/api/users?search=${encodeURIComponent(searchQuery(search))}`)
        .then((list) => {
          setPeople(list);
          setError("");
        })
        .catch((err) => setError(err.message));
    }, 250);
    return () => clearTimeout(timer);
  }, [search]);

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
