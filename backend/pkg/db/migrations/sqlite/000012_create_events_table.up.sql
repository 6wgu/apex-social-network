-- Group events. Readable by the group's members only. event_time is UTC in
-- SQLite's format ("2026-11-01 17:00:00").
CREATE TABLE IF NOT EXISTS events (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    group_id    INTEGER  NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    creator_id  INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT     NOT NULL CHECK (title <> ''),
    description TEXT     NOT NULL CHECK (description <> ''),
    event_time  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- A group's events in date order.
CREATE INDEX IF NOT EXISTS idx_events_group ON events(group_id, event_time);
