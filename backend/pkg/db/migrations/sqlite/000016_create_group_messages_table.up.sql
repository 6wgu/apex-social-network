CREATE TABLE IF NOT EXISTS group_messages (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    group_id   INTEGER  NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    sender_id  INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT     NOT NULL CHECK (content <> ''),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_group_messages_group ON group_messages(group_id, id);
