-- Group chat. Only members can send (checked inside the INSERT in
-- pkg/handlers/group_messages.go) or read. The recipients of a message are
-- read from group_members when it is sent, so there are no chat "rooms" kept
-- in memory that could go stale. Members see the whole history, including
-- what was said before they joined.
CREATE TABLE IF NOT EXISTS group_messages (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    group_id   INTEGER  NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    sender_id  INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT     NOT NULL CHECK (content <> ''),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- A group's chat, newest first.
CREATE INDEX IF NOT EXISTS idx_group_messages_group ON group_messages(group_id, id);
