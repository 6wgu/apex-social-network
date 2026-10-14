-- Pending requests to join a group. Only the group's creator can accept or
-- decline; either answer (or the requester cancelling) deletes the row.
-- One request per user and group at a time.
CREATE TABLE IF NOT EXISTS group_requests (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    group_id   INTEGER  NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (group_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_group_requests_user ON group_requests(user_id);
