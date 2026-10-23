CREATE TABLE IF NOT EXISTS notifications (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT     NOT NULL,
    actor_id   INTEGER  REFERENCES users(id) ON DELETE CASCADE,
    ref_id     INTEGER,
    is_read    INTEGER  NOT NULL DEFAULT 0 CHECK (is_read IN (0, 1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read);
