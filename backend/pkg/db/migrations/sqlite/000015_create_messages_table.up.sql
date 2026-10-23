CREATE TABLE IF NOT EXISTS messages (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    sender_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recipient_id INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content      TEXT     NOT NULL CHECK (content <> ''),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    read_at      DATETIME,
    CHECK (sender_id <> recipient_id)
);

CREATE INDEX IF NOT EXISTS idx_messages_sender_recipient ON messages(sender_id, recipient_id, id);
CREATE INDEX IF NOT EXISTS idx_messages_recipient_sender ON messages(recipient_id, sender_id, id);
CREATE INDEX IF NOT EXISTS idx_messages_unread ON messages(recipient_id, sender_id) WHERE read_at IS NULL;
