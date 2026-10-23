UPDATE users SET nickname = trim(nickname) WHERE nickname <> trim(nickname);

UPDATE users SET nickname = ''
WHERE nickname <> '' AND EXISTS (
    SELECT 1 FROM users older
    WHERE lower(older.nickname) = lower(users.nickname) AND older.id < users.id
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_nickname ON users(lower(nickname))
WHERE nickname IS NOT NULL AND nickname <> '';
