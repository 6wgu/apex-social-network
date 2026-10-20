-- Nicknames cleared by the up migration stay empty: there is no record of them.
DROP INDEX IF EXISTS idx_users_nickname;
