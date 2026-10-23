-- The token a correct password reset code is traded for. Only its SHA-256
-- hash is stored; it works once, until expires_at, and a user has at most
-- one.
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    token_hash TEXT     PRIMARY KEY,
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
