// Package seed fills an empty database with demo accounts and content, so
// every feature can be tried right away without registering or reading an
// email code. It runs on the first start, when the users table is empty,
// and never inside go test.
package seed

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Password is the password of every demo account. It passes the register
// rules (8+ characters, upper and lower case, a number, a special character).
const Password = "Audit123!"

// The demo images, copied into the upload folder under the same names.
//
//go:embed images
var images embed.FS

// sqliteTime is the format SQLite uses for DATETIME columns.
const sqliteTime = "2006-01-02 15:04:05"

// Enabled reads the SEED_DEMO setting: on unless it is 0, false, off or no.
func Enabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

// Emails lists the demo accounts' addresses. Reset removes exactly these.
func Emails() []string {
	emails := make([]string, len(users))
	for i, u := range users {
		emails[i] = u.Email
	}
	return emails
}

// Run seeds db when it has no users yet and reports whether it did. Inside
// go test it does nothing, so tests always start from an empty database.
func Run(db *sql.DB, uploadDir string) (bool, error) {
	if testing.Testing() {
		return false, nil
	}
	return run(db, uploadDir, time.Now())
}

// run does the work of Run; now anchors every date, so "yesterday" and
// "in three days" stay relative to the moment of seeding.
func run(db *sql.DB, uploadDir string, now time.Time) (bool, error) {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	if err := copyImages(uploadDir); err != nil {
		return false, fmt.Errorf("copy demo images: %w", err)
	}
	// One hash for all accounts: they share the password, and bcrypt is slow
	// on purpose.
	hash, err := bcrypt.GenerateFromPassword([]byte(Password), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	s := &seeder{tx: tx, now: now.UTC(), users: map[string]int64{}, posts: map[string]int64{}}
	for _, step := range []func() error{
		func() error { return s.insertUsers(string(hash)) },
		s.insertFollows,
		s.insertPosts,
		s.insertComments,
		s.insertLikes,
	} {
		if err := step(); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// Reset deletes the demo accounts and everything that hangs off them
// (posts, comments, likes, follows, chats, notifications, the groups they
// created) through the ON DELETE CASCADE foreign keys, then removes the demo
// images from the upload folder. Other accounts stay. It returns how many
// demo accounts it deleted.
func Reset(db *sql.DB, uploadDir string) (int64, error) {
	emails := Emails()
	args := make([]any, len(emails))
	for i, e := range emails {
		args[i] = e
	}
	// The placeholders are generated; every value is a bound parameter.
	res, err := db.Exec(`DELETE FROM users WHERE email IN (?`+strings.Repeat(", ?", len(emails)-1)+`)`, args...)
	if err != nil {
		return 0, err
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	names, err := fs.ReadDir(images, "images")
	if err != nil {
		return deleted, err
	}
	for _, img := range names {
		if err := os.Remove(filepath.Join(uploadDir, img.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return deleted, err
		}
	}
	return deleted, nil
}

// copyImages copies each demo image into uploadDir when it is not there yet.
func copyImages(uploadDir string) error {
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return err
	}
	names, err := fs.ReadDir(images, "images")
	if err != nil {
		return err
	}
	for _, img := range names {
		dest := filepath.Join(uploadDir, img.Name())
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		data, err := images.ReadFile(path.Join("images", img.Name()))
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// seeder inserts the dataset inside one transaction and remembers the ids
// it got, so later rows can refer to earlier ones by key.
type seeder struct {
	tx    *sql.Tx
	now   time.Time
	users map[string]int64
	posts map[string]int64
}

// at formats now shifted by d (negative for the past) as a DATETIME.
func (s *seeder) at(d time.Duration) string { return s.now.Add(d).Format(sqliteTime) }

// image returns the upload path of a demo image, or "" for none.
func image(name string) string {
	if name == "" {
		return ""
	}
	return "/uploads/" + name
}

func (s *seeder) insertUsers(hash string) error {
	for _, u := range users {
		public := 0
		if u.Public {
			public = 1
		}
		res, err := s.tx.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth,
			avatar_path, nickname, about_me, is_public, is_verified, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
			u.Email, hash, u.First, u.Last, u.Birth, image(u.Avatar), u.Nickname, u.About, public, s.at(-30*24*time.Hour))
		if err != nil {
			return fmt.Errorf("user %s: %w", u.Key, err)
		}
		if s.users[u.Key], err = res.LastInsertId(); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) insertFollows() error {
	for _, f := range follows {
		status := "accepted"
		if f.Pending {
			status = "pending"
		}
		if _, err := s.tx.Exec(`INSERT INTO follows (follower_id, following_id, status, created_at) VALUES (?, ?, ?, ?)`,
			s.users[f.From], s.users[f.To], status, s.at(-f.Ago)); err != nil {
			return fmt.Errorf("follow %s -> %s: %w", f.From, f.To, err)
		}
	}
	return nil
}

func (s *seeder) insertPosts() error {
	for _, p := range posts {
		res, err := s.tx.Exec(`INSERT INTO posts (user_id, content, image_path, privacy, created_at) VALUES (?, ?, ?, ?, ?)`,
			s.users[p.Author], p.Content, image(p.Image), p.Privacy, s.at(-p.Ago))
		if err != nil {
			return fmt.Errorf("post %s: %w", p.Key, err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		s.posts[p.Key] = id
		for _, v := range p.Viewers {
			if _, err := s.tx.Exec(`INSERT INTO post_viewers (post_id, user_id) VALUES (?, ?)`, id, s.users[v]); err != nil {
				return fmt.Errorf("post %s viewer %s: %w", p.Key, v, err)
			}
		}
	}
	return nil
}

func (s *seeder) insertComments() error {
	for _, c := range comments {
		if _, err := s.tx.Exec(`INSERT INTO comments (post_id, user_id, content, image_path, created_at) VALUES (?, ?, ?, ?, ?)`,
			s.posts[c.Post], s.users[c.Author], c.Content, image(c.Image), s.at(-c.Ago)); err != nil {
			return fmt.Errorf("comment on %s by %s: %w", c.Post, c.Author, err)
		}
	}
	return nil
}

func (s *seeder) insertLikes() error {
	for _, l := range likes {
		for _, u := range l.By {
			if _, err := s.tx.Exec(`INSERT INTO post_likes (post_id, user_id) VALUES (?, ?)`, s.posts[l.Post], s.users[u]); err != nil {
				return fmt.Errorf("like on %s by %s: %w", l.Post, u, err)
			}
		}
	}
	return nil
}
