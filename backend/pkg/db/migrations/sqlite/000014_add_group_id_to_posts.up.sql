-- Group posts are ordinary posts with group_id set. They are visible to the
-- group's members only, whatever their privacy column says (group posts
-- store 'public'); see postVisibleSQL in pkg/handlers/posts.go. Profile
-- posts keep group_id NULL. Comments and images of a group post follow the
-- post, so they are members-only too.
ALTER TABLE posts ADD COLUMN group_id INTEGER REFERENCES groups(id) ON DELETE CASCADE;

-- A group's feed, newest first.
CREATE INDEX IF NOT EXISTS idx_posts_group ON posts(group_id, id) WHERE group_id IS NOT NULL;
