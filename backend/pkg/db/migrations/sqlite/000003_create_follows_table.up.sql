CREATE TABLE IF NOT EXISTS follows (
    follower_id  INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    following_id INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT     NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted')),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (follower_id, following_id),
    CHECK (follower_id <> following_id)
);

CREATE INDEX IF NOT EXISTS idx_follows_following_status ON follows(following_id, status);
