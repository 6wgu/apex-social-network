-- Groups. Every logged-in user can browse them (title, description,
-- creator, member count); what happens inside a group (members, posts,
-- events) is for its members only. The creator is also a row in
-- group_members, added together with the group.
CREATE TABLE IF NOT EXISTS groups (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    creator_id  INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT     NOT NULL CHECK (title <> ''),
    description TEXT     NOT NULL CHECK (description <> ''),
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_groups_creator ON groups(creator_id);
