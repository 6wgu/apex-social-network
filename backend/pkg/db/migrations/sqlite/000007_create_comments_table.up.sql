-- Comments on posts. A comment is readable by exactly the users who can see
-- its post (CanViewPost), and so is its image.
CREATE TABLE IF NOT EXISTS comments (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    post_id    INTEGER  NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT     NOT NULL DEFAULT '',
    image_path TEXT     NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (content <> '' OR image_path <> '')
);

-- A post's comments in order, and its comment count.
CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id, id);
-- GET /uploads/{name} looks up which comment (if any) a file belongs to.
CREATE INDEX IF NOT EXISTS idx_comments_image ON comments(image_path) WHERE image_path <> '';
