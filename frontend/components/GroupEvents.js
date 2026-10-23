"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { formatEventTime, parseTime } from "@/lib/time";
import { Loading } from "@/components/RacingLights";

function localInputValue(date) {
  const pad = (n) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function EventForm({ groupId, onCreated, onCancel }) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [when, setWhen] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e) {
    e.preventDefault();
    setError("");
    const date = new Date(when);
    if (Number.isNaN(date.getTime())) {
      setError("Choose a date and time.");
      return;
    }
    setBusy(true);
    try {
      const event = await api(`/api/groups/${groupId}/events`, {
        method: "POST",
        body: { title, description, event_time: date.toISOString() },
      });
      onCreated(event);
    } catch (err) {
      setError(err.message);
      setBusy(false);
    }
  }

  return (
    <form className="form event-form" onSubmit={submit}>
      <label>
        Title
        <input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={100} required />
      </label>
      <label>
        Description
        <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} maxLength={2000} required />
      </label>
      <label>
        Day and time
        <input
          type="datetime-local"
          value={when}
          min={localInputValue(new Date())}
          onChange={(e) => setWhen(e.target.value)}
          required
        />
      </label>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="form-actions">
        <button type="button" className="btn-secondary" onClick={onCancel} disabled={busy}>
          Cancel
        </button>
        <button className="btn-primary" disabled={busy || !title.trim() || !description.trim() || !when}>
          {busy ? "Creating…" : "Create event"}
        </button>
      </div>
    </form>
  );
}

function EventCard({ event, onChange }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const past = parseTime(event.event_time) < new Date();

  async function respond(response) {
    if (event.my_response === response) return;
    setBusy(true);
    setError("");
    try {
      onChange(await api(`/api/events/${event.id}/respond`, { method: "POST", body: { response } }));
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <li className={`event${past ? " event-past" : ""}`}>
      <div className="event-date" aria-hidden="true">
        <span>{parseTime(event.event_time).toLocaleDateString(undefined, { month: "short" })}</span>
        <strong>{parseTime(event.event_time).getDate()}</strong>
      </div>
      <div className="event-body">
        <h3>{event.title}</h3>
        <p className="muted event-when">
          🕒 <time dateTime={parseTime(event.event_time).toISOString()}>{formatEventTime(event.event_time)}</time>
          {past && <span className="pill pill-private">Past</span>}
        </p>
        <p className="event-desc">{event.description}</p>
        <p className="muted event-by">
          Created by{" "}
          <Link href={`/profile/${event.creator.id}`}>
            {event.creator.first_name} {event.creator.last_name}
          </Link>
        </p>
        <div className="rsvp" role="group" aria-label="Your answer">
          <button
            className={`rsvp-btn${event.my_response === "going" ? " rsvp-going" : ""}`}
            aria-pressed={event.my_response === "going"}
            disabled={busy}
            onClick={() => respond("going")}
          >
            ✅ Going <span className="rsvp-count">{event.going_count}</span>
          </button>
          <button
            className={`rsvp-btn${event.my_response === "not_going" ? " rsvp-not-going" : ""}`}
            aria-pressed={event.my_response === "not_going"}
            disabled={busy}
            onClick={() => respond("not_going")}
          >
            ❌ Not going <span className="rsvp-count">{event.not_going_count}</span>
          </button>
        </div>
        {error && <p className="form-error">{error}</p>}
      </div>
    </li>
  );
}

export default function GroupEvents({ groupId, reloadKey }) {
  const [events, setEvents] = useState(null);
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);

  const load = useCallback(
    () =>
      api(`/api/groups/${groupId}/events`)
        .then((list) => {
          setEvents(list);
          setError("");
        })
        .catch((err) => setError(err.message)),
    [groupId]
  );

  useEffect(() => {
    load();
  }, [load, reloadKey]);

  const replace = (event) => setEvents((list) => list.map((e) => (e.id === event.id ? event : e)));
  function created(event) {
    setCreating(false);
    setEvents((list) => [...(list || []), event].sort((a, b) => parseTime(a.event_time) - parseTime(b.event_time)));
  }

  const now = new Date();
  const upcoming = (events || []).filter((e) => parseTime(e.event_time) >= now);
  const past = (events || []).filter((e) => parseTime(e.event_time) < now).reverse();

  return (
    <div className="group-events">
      {creating ? (
        <EventForm groupId={groupId} onCreated={created} onCancel={() => setCreating(false)} />
      ) : (
        <div className="tab-toolbar">
          <button className="btn-primary" onClick={() => setCreating(true)}>
            + New event
          </button>
        </div>
      )}
      {error && <p className="form-error">{error}</p>}
      {!events && !error && <Loading className="muted tab-placeholder">Loading events…</Loading>}
      {events && events.length === 0 && <p className="muted tab-placeholder">No events yet. Plan the first one!</p>}
      {upcoming.length > 0 && (
        <>
          <h3 className="list-heading">Upcoming</h3>
          <ul className="event-list">
            {upcoming.map((e) => (
              <EventCard key={e.id} event={e} onChange={replace} />
            ))}
          </ul>
        </>
      )}
      {past.length > 0 && (
        <>
          <h3 className="list-heading">Past</h3>
          <ul className="event-list">
            {past.map((e) => (
              <EventCard key={e.id} event={e} onChange={replace} />
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
