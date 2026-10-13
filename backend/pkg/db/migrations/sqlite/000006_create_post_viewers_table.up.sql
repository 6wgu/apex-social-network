-- The followers an author chose for a 'private' post. Rows only exist for
-- private posts; they disappear with the post or the user.
CREATE TABLE IF NOT EXISTS post_viewers (
    post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, user_id)
);

-- "Is this user a chosen viewer?" is asked per user, across posts (feed).
CREATE INDEX IF NOT EXISTS idx_post_viewers_user ON post_viewers(user_id, post_id);
