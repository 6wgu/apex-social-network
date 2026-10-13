-- One row per notification shown in a user's bell.
-- type is checked in Go (pkg/handlers/notifications.go), not with a CHECK,
-- so Phase 3 can add group_invite, group_join_request and group_event
-- without rebuilding the table.
-- ref_id points at the thing the notification is about; what it refers to
-- depends on type (NULL for the follow types, where actor_id says it all;
-- a group or event id for the Phase 3 types).
CREATE TABLE IF NOT EXISTS notifications (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT     NOT NULL,
    actor_id   INTEGER  REFERENCES users(id) ON DELETE CASCADE,
    ref_id     INTEGER,
    is_read    INTEGER  NOT NULL DEFAULT 0 CHECK (is_read IN (0, 1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- The bell lists a user's notifications newest first and counts unread ones.
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id, id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id, is_read);
