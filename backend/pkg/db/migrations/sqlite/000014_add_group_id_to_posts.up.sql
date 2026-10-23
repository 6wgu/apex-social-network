ALTER TABLE posts ADD COLUMN group_id INTEGER REFERENCES groups(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_posts_group ON posts(group_id, id) WHERE group_id IS NOT NULL;
