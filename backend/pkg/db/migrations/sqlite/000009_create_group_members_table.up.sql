-- Who is in which group. A row is added when a group is created (the
-- creator), when an invitation is accepted, or when the creator accepts a
-- join request.
CREATE TABLE IF NOT EXISTS group_members (
    group_id  INTEGER  NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id)
);

-- "Which groups is this user in?" (chat, Phase 4).
CREATE INDEX IF NOT EXISTS idx_group_members_user ON group_members(user_id, group_id);
