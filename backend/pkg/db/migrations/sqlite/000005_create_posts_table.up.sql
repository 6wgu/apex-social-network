CREATE TABLE IF NOT EXISTS posts (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT     NOT NULL DEFAULT '',
    image_path TEXT     NOT NULL DEFAULT '',
    privacy    TEXT     NOT NULL CHECK (privacy IN ('public', 'almost_private', 'private')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (content <> '' OR image_path <> '')
);

CREATE INDEX IF NOT EXISTS idx_posts_user ON posts(user_id, id);
CREATE INDEX IF NOT EXISTS idx_posts_image ON posts(image_path) WHERE image_path <> '';
