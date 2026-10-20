"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import useSearchQuery from "@/lib/useSearchQuery";
import Avatar from "@/components/Avatar";
import Modal from "@/components/Modal";
import SearchInput from "@/components/SearchInput";

// "Invite people" for a group member. It lists the people I follow and my
// followers; typing searches every user (GET /api/users?search=). Members
// and people already invited are marked instead of offering the button.
// The server checks it all again (members only, no duplicates).
export default function InviteDialog({ group, me, onClose, onInvited }) {
  const [contacts, setContacts] = useState(null);
  const [search, setSearch] = useState("");
  const query = useSearchQuery(search);
  const [results, setResults] = useState(null);
  const [invited, setInvited] = useState(() => new Set((group.invited_users || []).map((u) => u.id)));
  const [busy, setBusy] = useState(null);
  const [error, setError] = useState("");
  const members = new Set((group.members || []).map((u) => u.id));

  // Followers and following, merged and sorted by name.
  useEffect(() => {
    Promise.all([api(`/api/users/${me.id}/followers`), api(`/api/users/${me.id}/following`)])
      .then(([followers, following]) => {
        const byId = new Map([...followers, ...following].map((u) => [u.id, u]));
        const list = [...byId.values()].sort((a, b) =>
          `${a.first_name} ${a.last_name}`.localeCompare(`${b.first_name} ${b.last_name}`)
        );
        setContacts(list);
      })
      .catch((err) => setError(err.message));
  }, [me.id]);

  // The query settles once typing pauses; only the latest answer is shown.
  useEffect(() => {
    if (!query) {
      setResults(null);
      return;
    }
    let current = true;
    api(`/api/users?search=${encodeURIComponent(query)}`)
      .then((list) => current && setResults(list.filter((u) => u.id !== me.id)))
      .catch((err) => current && setError(err.message));
    return () => {
      current = false;
    };
  }, [query, me.id]);

  async function inviteUser(user) {
    setBusy(user.id);
    setError("");
    try {
      await api(`/api/groups/${group.id}/invite`, { method: "POST", body: { user_id: user.id } });
      setInvited((s) => new Set(s).add(user.id));
      onInvited();
    } catch (err) {
      if (err.status === 409) {
        setInvited((s) => new Set(s).add(user.id)); // invited or joined meanwhile
        onInvited();
      } else {
        setError(err.message);
      }
    } finally {
      setBusy(null);
    }
  }

  const people = results ?? contacts;

  return (
    <Modal title={`Invite people to ${group.title}`} onClose={onClose}>
      <SearchInput
        className="search"
        placeholder="Search everyone by name or nickname"
        value={search}
        onChange={setSearch}
        aria-label="Search people to invite"
      />
      <p className="hint invite-hint">{results ? "Search results" : "People you follow and your followers"}</p>
      {error && <p className="form-error">{error}</p>}
      {!people && !error && <p className="muted">Loading…</p>}
      {people && people.length === 0 && (
        <p className="muted">
          {results
            ? "Nobody matches that search."
            : "You don't follow anyone and have no followers yet. Search for people above."}
        </p>
      )}
      <ul className="invite-list">
        {(people || []).map((u) => (
          <li key={u.id}>
            <Avatar user={u} size={36} />
            <span className="invite-name">
              <strong>
                {u.first_name} {u.last_name}
              </strong>
              {u.nickname && <span className="muted"> @{u.nickname}</span>}
            </span>
            {members.has(u.id) ? (
              <span className="muted">Member</span>
            ) : invited.has(u.id) ? (
              <span className="muted">Invited ✓</span>
            ) : (
              <button className="btn-primary" disabled={busy === u.id} onClick={() => inviteUser(u)}>
                Invite
              </button>
            )}
          </li>
        ))}
      </ul>
    </Modal>
  );
}
