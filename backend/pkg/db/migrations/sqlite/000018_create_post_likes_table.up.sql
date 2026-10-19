-- Likes on posts: one row per user who liked a post. Only users who can see
-- the post (CanViewPost) may like it; that is checked inside the INSERT.
CREATE TABLE IF NOT EXISTS post_likes (
    post_id    INTEGER  NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (post_id, user_id)
);
