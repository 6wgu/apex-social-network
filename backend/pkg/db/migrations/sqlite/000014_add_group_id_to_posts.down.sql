DROP INDEX IF EXISTS idx_posts_group;
DELETE FROM posts WHERE group_id IS NOT NULL;
ALTER TABLE posts DROP COLUMN group_id;
