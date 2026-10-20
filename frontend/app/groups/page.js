"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import useSearchQuery from "@/lib/useSearchQuery";
import GroupJoinButton from "@/components/GroupJoinButton";
import Modal from "@/components/Modal";
import { Loading } from "@/components/RacingLights";
import { GROUPS_CHANGED } from "@/components/NotificationBell";
import { useRealtime } from "@/components/RealtimeProvider";
import SearchInput from "@/components/SearchInput";

// Form for POST /api/groups. On success it opens the new group.
function CreateGroup({ onClose }) {
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const group = await api("/api/groups", { method: "POST", body: { title, description } });
      router.push(`/groups/${group.id}`);
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  }

  return (
    <Modal title="Create a group" onClose={onClose}>
      <form className="form" onSubmit={submit}>
        <label>
          Title
          <input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={100} required />
        </label>
        <label>
          Description
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={4} maxLength={2000} required />
          <span className="hint">What is the group about? Everyone can read this before joining.</span>
        </label>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <div className="form-actions">
          <button type="button" className="btn-secondary" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button className="btn-primary" disabled={busy || !title.trim() || !description.trim()}>
            {busy ? "Creating…" : "Create group"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// One group in a list: initial, title, description, member count, creator,
// and the action that fits my status.
function GroupRow({ group, onChange }) {
  const href = `/groups/${group.id}`;
  return (
    <li className="group-row">
      <Link href={href} className="group-icon" aria-hidden="true" tabIndex={-1}>
        {group.title[0].toUpperCase()}
      </Link>
      <div className="group-info">
        <Link href={href} className="group-title">
          {group.title}
        </Link>
        <p className="group-desc">{group.description}</p>
        <p className="muted group-meta">
          {group.member_count} {group.member_count === 1 ? "member" : "members"} · created by {group.creator.first_name}{" "}
          {group.creator.last_name}
        </p>
      </div>
      {group.my_status === "member" ? (
        <Link href={href} className="btn-secondary">
          Open
        </Link>
      ) : (
        <GroupJoinButton group={group} onChange={onChange} />
      )}
    </li>
  );
}

function Section({ title, groups, empty, onChange }) {
  return (
    <section className="card">
      <h2>{title}</h2>
      {groups.length === 0 ? (
        <p className="muted">{empty}</p>
      ) : (
        <ul className="group-list">
          {groups.map((g) => (
            <GroupRow key={g.id} group={g} onChange={onChange} />
          ))}
        </ul>
      )}
    </section>
  );
}

// /groups: every group (GET /api/groups), split into my invitations, my
// groups, and the rest to discover, with a filter and "Create group".
export default function GroupsPage() {
  const [groups, setGroups] = useState(null);
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const [search, setSearch] = useState("");
  const q = useSearchQuery(search).toLowerCase();

  const load = useCallback(
    () =>
      api("/api/groups")
        .then((list) => {
          setGroups(list);
          setError("");
        })
        .catch((err) => setError(err.message)),
    []
  );

  useEffect(() => {
    load();
  }, [load]);
  // Invitations and accepted requests arrive in the bell; keep the lists current.
  useEffect(() => {
    window.addEventListener(GROUPS_CHANGED, load);
    return () => window.removeEventListener(GROUPS_CHANGED, load);
  }, [load]);
  useRealtime((event) => {
    if (event.type === "notification" && event.payload.group) load();
  });

  const shown = (groups || []).filter((g) => !q || g.title.toLowerCase().includes(q) || g.description.toLowerCase().includes(q));
  const invited = shown.filter((g) => g.my_status === "invited");
  const mine = shown.filter((g) => g.my_status === "member");
  const others = shown.filter((g) => g.my_status === "none" || g.my_status === "requested");

  return (
    <div className="groups-page">
      <section className="card groups-head">
        <div>
          <h1>Groups</h1>
          <p className="muted">Join groups to share posts and plan events with their members.</p>
        </div>
        <button className="btn-primary" onClick={() => setCreating(true)}>
          + Create group
        </button>
        <SearchInput
          className="search"
          placeholder="Filter groups by title or description"
          value={search}
          onChange={setSearch}
          aria-label="Filter groups"
        />
      </section>

      {error && <p className="form-error">{error}</p>}
      {!groups && !error && <Loading>Loading groups…</Loading>}
      {groups && (
        <>
          {invited.length > 0 && <Section title="Invitations" groups={invited} onChange={load} />}
          <Section
            title="Your groups"
            groups={mine}
            onChange={load}
            empty={q ? "None of your groups match." : "You are not in any group yet. Create one, or ask to join one below."}
          />
          <Section
            title="Discover"
            groups={others}
            onChange={load}
            empty={q ? "No other group matches." : "There are no other groups yet."}
          />
        </>
      )}

      {creating && <CreateGroup onClose={() => setCreating(false)} />}
    </div>
  );
}
