CREATE TABLE IF NOT EXISTS comments (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    post_id    INTEGER  NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT     NOT NULL DEFAULT '',
    image_path TEXT     NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (content <> '' OR image_path <> '')
);

CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id, id);
CREATE INDEX IF NOT EXISTS idx_comments_image ON comments(image_path) WHERE image_path <> '';
