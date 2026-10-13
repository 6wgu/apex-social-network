CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT    NOT NULL,
    first_name    TEXT    NOT NULL,
    last_name     TEXT    NOT NULL,
    date_of_birth TEXT    NOT NULL,
    avatar_path   TEXT    NOT NULL DEFAULT '',
    nickname      TEXT    NOT NULL DEFAULT '',
    about_me      TEXT    NOT NULL DEFAULT '',
    is_public     INTEGER NOT NULL DEFAULT 1 CHECK (is_public IN (0, 1)),
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
