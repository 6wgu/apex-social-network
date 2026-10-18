CREATE TABLE IF NOT EXISTS otp_codes (
    subject      TEXT     PRIMARY KEY,
    code_hash    TEXT     NOT NULL,
    salt         TEXT     NOT NULL,
    expires_at   DATETIME NOT NULL,
    attempts     INTEGER  NOT NULL DEFAULT 0,
    last_sent_at DATETIME NOT NULL
);

ALTER TABLE users ADD COLUMN is_verified INTEGER NOT NULL DEFAULT 0 CHECK (is_verified IN (0, 1));
