CREATE TABLE IF NOT EXISTS group_invites (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    group_id   INTEGER  NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    inviter_id INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_id INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (group_id, invitee_id),
    CHECK (inviter_id <> invitee_id)
);

CREATE INDEX IF NOT EXISTS idx_group_invites_invitee ON group_invites(invitee_id);
