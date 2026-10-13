-- Profile posts. Who may read one is decided by privacy (see CanViewPost in
-- pkg/handlers/posts.go):
--   public          every logged-in user
--   almost_private  the author's accepted followers
--   private         the followers chosen in post_viewers
-- Group posts (Phase 3) will add a group_id column with ALTER TABLE.
CREATE TABLE IF NOT EXISTS posts (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT     NOT NULL DEFAULT '',
    image_path TEXT     NOT NULL DEFAULT '',
    privacy    TEXT     NOT NULL CHECK (privacy IN ('public', 'almost_private', 'private')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (content <> '' OR image_path <> '')
);

-- A profile's posts, newest first.
CREATE INDEX IF NOT EXISTS idx_posts_user ON posts(user_id, id);
-- GET /uploads/{name} looks up which post (if any) a file belongs to.
CREATE INDEX IF NOT EXISTS idx_posts_image ON posts(image_path) WHERE image_path <> '';
