-- A nickname is unique when it is set, without regard to case: "Ahmed",
-- "ahmed" and "AHMED" are the same. No nickname is stored as '' (the
-- column is NOT NULL), and the index leaves those rows out, so any number
-- of users can have none.
--
-- Nicknames that are already shared: the oldest account (lowest id) keeps
-- it and the later ones get an empty nickname, which they can set again in
-- Settings. Nothing else about those users changes.
UPDATE users SET nickname = trim(nickname) WHERE nickname <> trim(nickname);

UPDATE users SET nickname = ''
WHERE nickname <> '' AND EXISTS (
    SELECT 1 FROM users older
    WHERE lower(older.nickname) = lower(users.nickname) AND older.id < users.id
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_nickname ON users(lower(nickname))
WHERE nickname IS NOT NULL AND nickname <> '';
