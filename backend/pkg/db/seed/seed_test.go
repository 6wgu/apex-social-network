package seed

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"social-network/pkg/db/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.MigrateUp(db, "file://../migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	return db
}

func count(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func TestRunIsSkippedInTests(t *testing.T) {
	db := newTestDB(t)
	if seeded, err := Run(db, t.TempDir()); err != nil || seeded {
		t.Fatalf("Run = %v, %v; want nothing inside go test", seeded, err)
	}
	if n := count(t, db, `SELECT COUNT(*) FROM users`); n != 0 {
		t.Fatalf("users = %d, want 0", n)
	}
}

func TestSeedRunsOnceAndResets(t *testing.T) {
	db := newTestDB(t)
	uploads := t.TempDir()
	seeded, err := run(db, uploads, time.Now())
	if err != nil || !seeded {
		t.Fatalf("first run = %v, %v", seeded, err)
	}

	for query, want := range map[string]int{
		`SELECT COUNT(*) FROM users`:                        len(users),
		`SELECT COUNT(*) FROM follows`:                      len(follows),
		`SELECT COUNT(*) FROM posts WHERE group_id IS NULL`: len(posts),
		`SELECT COUNT(*) FROM comments c JOIN posts p ON p.id = c.post_id WHERE p.group_id IS NULL`: len(comments),
	} {
		if n := count(t, db, query); n != want {
			t.Errorf("%s = %d, want %d", query, n, want)
		}
	}
	if n := count(t, db, `SELECT COUNT(*) FROM users WHERE is_verified = 1`); n != len(users) {
		t.Errorf("verified users = %d, want all %d", n, len(users))
	}
	for _, privacy := range []string{"public", "almost_private", "private"} {
		if count(t, db, `SELECT COUNT(*) FROM posts WHERE privacy = ? AND group_id IS NULL`, privacy) == 0 {
			t.Errorf("no %s post", privacy)
		}
	}
	if count(t, db, `SELECT COUNT(*) FROM follows WHERE status = 'pending'`) == 0 {
		t.Error("no pending follow request")
	}
	// Every image the database points at was copied into the upload folder.
	rows, err := db.Query(`SELECT avatar_path FROM users UNION SELECT image_path FROM posts UNION SELECT image_path FROM comments`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var p string
		rows.Scan(&p)
		if p == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(uploads, filepath.Base(p))); err != nil {
			t.Errorf("%s not in uploads: %v", p, err)
		}
	}
	rows.Close()

	// The data follows the app's own rules: a private post's viewers follow
	// its author, and whoever commented on or liked a post can see it.
	if n := count(t, db, `SELECT COUNT(*) FROM post_viewers v JOIN posts p ON p.id = v.post_id
		WHERE NOT EXISTS (SELECT 1 FROM follows f WHERE f.follower_id = v.user_id AND f.following_id = p.user_id AND f.status = 'accepted')`); n != 0 {
		t.Errorf("%d private-post viewers do not follow the author", n)
	}
	const canSee = `(p.user_id = x.user_id OR p.privacy = 'public'
		OR (p.privacy = 'almost_private' AND EXISTS (SELECT 1 FROM follows f WHERE f.follower_id = x.user_id AND f.following_id = p.user_id AND f.status = 'accepted'))
		OR (p.privacy = 'private' AND EXISTS (SELECT 1 FROM post_viewers v WHERE v.post_id = p.id AND v.user_id = x.user_id)))`
	for _, table := range []string{"comments", "post_likes"} {
		if n := count(t, db, `SELECT COUNT(*) FROM `+table+` x JOIN posts p ON p.id = x.post_id WHERE NOT `+canSee); n != 0 {
			t.Errorf("%d %s on posts their author cannot see", n, table)
		}
	}

	// Groups, events and chats follow the same rules as the API.
	for table, want := range map[string]int{"groups": len(groups), "events": 2, "group_invites": 2, "group_requests": 2, "messages": len(messages)} {
		if n := count(t, db, `SELECT COUNT(*) FROM `+table); n != want {
			t.Errorf("%s = %d, want %d", table, n, want)
		}
	}
	if n := count(t, db, `SELECT COUNT(*) FROM messages m WHERE NOT EXISTS (SELECT 1 FROM follows f WHERE f.status = 'accepted'
		AND ((f.follower_id = m.sender_id AND f.following_id = m.recipient_id) OR (f.follower_id = m.recipient_id AND f.following_id = m.sender_id)))`); n != 0 {
		t.Errorf("%d private messages between users who do not follow each other", n)
	}
	for _, q := range []string{
		`SELECT COUNT(*) FROM posts p WHERE p.group_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = p.group_id AND m.user_id = p.user_id)`,
		`SELECT COUNT(*) FROM comments c JOIN posts p ON p.id = c.post_id WHERE p.group_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = p.group_id AND m.user_id = c.user_id)`,
		`SELECT COUNT(*) FROM group_messages x WHERE NOT EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = x.group_id AND m.user_id = x.sender_id)`,
		`SELECT COUNT(*) FROM event_responses r JOIN events e ON e.id = r.event_id WHERE NOT EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = e.group_id AND m.user_id = r.user_id)`,
		`SELECT COUNT(*) FROM group_invites i WHERE EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = i.group_id AND m.user_id = i.invitee_id)`,
		`SELECT COUNT(*) FROM group_requests r WHERE EXISTS (SELECT 1 FROM group_members m WHERE m.group_id = r.group_id AND m.user_id = r.user_id)`,
		`SELECT COUNT(*) FROM events WHERE datetime(event_time) <= datetime('now')`,
	} {
		if n := count(t, db, q); n != 0 {
			t.Errorf("%d rows break the rules: %s", n, q)
		}
	}
	for _, typ := range []string{"follow_request", "new_follower", "follow_accepted", "group_invite",
		"group_join_request", "group_request_accepted", "group_event"} {
		if count(t, db, `SELECT COUNT(*) FROM notifications WHERE type = ?`, typ) == 0 {
			t.Errorf("no %s notification", typ)
		}
	}
	// A follow_request notification exists exactly for each pending follow.
	if n := count(t, db, `SELECT COUNT(*) FROM notifications n WHERE n.type = 'follow_request' AND NOT EXISTS (
		SELECT 1 FROM follows f WHERE f.follower_id = n.actor_id AND f.following_id = n.user_id AND f.status = 'pending')`); n != 0 {
		t.Errorf("%d follow_request notifications without a pending follow", n)
	}

	// A second start does nothing.
	if seeded, err := run(db, uploads, time.Now()); err != nil || seeded {
		t.Fatalf("second run = %v, %v; want no-op", seeded, err)
	}

	// A real account survives the reset; the demo data and images do not.
	if _, err := db.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth)
		VALUES ('real@example.org', 'x', 'Real', 'Person', '1990-01-01')`); err != nil {
		t.Fatal(err)
	}
	deleted, err := Reset(db, uploads)
	if err != nil || deleted != int64(len(users)) {
		t.Fatalf("Reset = %d, %v; want %d", deleted, err, len(users))
	}
	if n := count(t, db, `SELECT COUNT(*) FROM users`); n != 1 {
		t.Errorf("users after reset = %d, want 1", n)
	}
	for _, table := range []string{"follows", "posts", "comments", "post_likes", "post_viewers", "groups", "group_members",
		"group_invites", "group_requests", "events", "event_responses", "messages", "group_messages", "notifications"} {
		if n := count(t, db, `SELECT COUNT(*) FROM `+table); n != 0 {
			t.Errorf("%s after reset = %d, want 0", table, n)
		}
	}
	if left, _ := os.ReadDir(uploads); len(left) != 0 {
		t.Errorf("%d demo images left in uploads", len(left))
	}
}

func TestEnabled(t *testing.T) {
	for value, want := range map[string]bool{"": true, "1": true, "true": true, "0": false, "false": false, " OFF ": false, "no": false} {
		if got := Enabled(value); got != want {
			t.Errorf("Enabled(%q) = %v, want %v", value, got, want)
		}
	}
}
