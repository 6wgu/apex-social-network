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
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const Password = "Audit123!"

//go:embed images
var images embed.FS

const sqliteTime = "2006-01-02 15:04:05"

func Enabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "false", "off", "no":
		return false
	}
	return true
}

func Emails() []string {
	emails := make([]string, len(users))
	for i, u := range users {
		emails[i] = u.Email
	}
	return emails
}

func Run(db *sql.DB, uploadDir string) (bool, error) {
	if testing.Testing() {
		return false, nil
	}
	return run(db, uploadDir, time.Now())
}

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
	hash, err := bcrypt.GenerateFromPassword([]byte(Password), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	s := &seeder{tx: tx, now: now.UTC(), users: map[string]int64{}, posts: map[string]int64{},
		groups: map[string]int64{}, events: map[string]int64{}}
	for _, step := range []func() error{
		func() error { return s.insertUsers(string(hash)) },
		s.insertFollows,
		s.insertPosts,
		s.insertComments,
		s.insertLikes,
		s.insertGroups,
		s.insertMessages,
		s.insertNotifications,
	} {
		if err := step(); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func Reset(db *sql.DB, uploadDir string) (int64, error) {
	emails := Emails()
	args := make([]any, len(emails))
	for i, e := range emails {
		args[i] = e
	}
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

type seeder struct {
	tx     *sql.Tx
	now    time.Time
	users  map[string]int64
	posts  map[string]int64
	groups map[string]int64
	events map[string]int64
}

func (s *seeder) at(d time.Duration) string { return s.now.Add(d).Format(sqliteTime) }

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

func (s *seeder) insertGroups() error {
	for _, g := range groups {
		res, err := s.tx.Exec(`INSERT INTO groups (creator_id, title, description, created_at) VALUES (?, ?, ?, ?)`,
			s.users[g.Creator], g.Title, g.Description, s.at(-g.Ago))
		if err != nil {
			return fmt.Errorf("group %s: %w", g.Key, err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		s.groups[g.Key] = id
		members := append([]member{{User: g.Creator, Ago: g.Ago}}, g.Members...)
		for _, m := range members {
			if _, err := s.tx.Exec(`INSERT INTO group_members (group_id, user_id, joined_at) VALUES (?, ?, ?)`,
				id, s.users[m.User], s.at(-m.Ago)); err != nil {
				return fmt.Errorf("group %s member %s: %w", g.Key, m.User, err)
			}
		}
		for _, inv := range g.Invites {
			if _, err := s.tx.Exec(`INSERT INTO group_invites (group_id, inviter_id, invitee_id, created_at) VALUES (?, ?, ?, ?)`,
				id, s.users[inv.From], s.users[inv.To], s.at(-day)); err != nil {
				return fmt.Errorf("group %s invite %s: %w", g.Key, inv.To, err)
			}
		}
		for _, u := range g.Requests {
			if _, err := s.tx.Exec(`INSERT INTO group_requests (group_id, user_id, created_at) VALUES (?, ?, ?)`,
				id, s.users[u], s.at(-4*hour)); err != nil {
				return fmt.Errorf("group %s request %s: %w", g.Key, u, err)
			}
		}
		for _, p := range g.Posts {
			res, err := s.tx.Exec(`INSERT INTO posts (user_id, content, image_path, privacy, group_id, created_at) VALUES (?, ?, ?, 'public', ?, ?)`,
				s.users[p.Author], p.Content, image(p.Image), id, s.at(-p.Ago))
			if err != nil {
				return fmt.Errorf("group %s post: %w", g.Key, err)
			}
			postID, err := res.LastInsertId()
			if err != nil {
				return err
			}
			for _, c := range p.Comments {
				if _, err := s.tx.Exec(`INSERT INTO comments (post_id, user_id, content, image_path, created_at) VALUES (?, ?, ?, ?, ?)`,
					postID, s.users[c.Author], c.Content, image(c.Image), s.at(-c.Ago)); err != nil {
					return fmt.Errorf("group %s comment by %s: %w", g.Key, c.Author, err)
				}
			}
		}
		for i, e := range g.Events {
			date := s.now.Add(e.In)
			when := time.Date(date.Year(), date.Month(), date.Day(), e.Hour, 0, 0, 0, time.UTC).Format(sqliteTime)
			res, err := s.tx.Exec(`INSERT INTO events (group_id, creator_id, title, description, event_time, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
				id, s.users[g.Creator], e.Title, e.Description, when, s.at(-day))
			if err != nil {
				return fmt.Errorf("group %s event %s: %w", g.Key, e.Title, err)
			}
			eventID, err := res.LastInsertId()
			if err != nil {
				return err
			}
			s.events[fmt.Sprintf("%s/%d", g.Key, i)] = eventID
			for response, people := range map[string][]string{"going": e.Going, "not_going": e.NotGoing} {
				for _, u := range people {
					if _, err := s.tx.Exec(`INSERT INTO event_responses (event_id, user_id, response) VALUES (?, ?, ?)`,
						eventID, s.users[u], response); err != nil {
						return fmt.Errorf("event %s response %s: %w", e.Title, u, err)
					}
				}
			}
		}
		for _, line := range g.Chat {
			if _, err := s.tx.Exec(`INSERT INTO group_messages (group_id, sender_id, content, created_at) VALUES (?, ?, ?, ?)`,
				id, s.users[line.From], line.Text, s.at(-line.Ago)); err != nil {
				return fmt.Errorf("group %s chat: %w", g.Key, err)
			}
		}
	}
	return nil
}

func (s *seeder) insertMessages() error {
	ordered := append([]message(nil), messages...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Ago > ordered[j].Ago })
	for _, m := range ordered {
		var readAt any
		if !m.Unread {
			readAt = s.at(-m.Ago + 10*time.Minute)
		}
		if _, err := s.tx.Exec(`INSERT INTO messages (sender_id, recipient_id, content, created_at, read_at) VALUES (?, ?, ?, ?, ?)`,
			s.users[m.From], s.users[m.To], m.Text, s.at(-m.Ago), readAt); err != nil {
			return fmt.Errorf("message %s -> %s: %w", m.From, m.To, err)
		}
	}
	return nil
}

func (s *seeder) insertNotifications() error {
	public := map[string]bool{}
	for _, u := range users {
		public[u.Key] = u.Public
	}
	type row struct {
		user, typ, actor string
		ref              any
		ago              time.Duration
	}
	var rows []row
	add := func(user, typ, actor string, ref any, ago time.Duration) {
		rows = append(rows, row{user, typ, actor, ref, ago})
	}
	for _, f := range follows {
		switch {
		case f.Pending:
			add(f.To, "follow_request", f.From, nil, f.Ago)
		case public[f.To]:
			add(f.To, "new_follower", f.From, nil, f.Ago)
		default:
			add(f.From, "follow_accepted", f.To, nil, f.Ago)
		}
	}
	for _, g := range groups {
		id := s.groups[g.Key]
		for _, inv := range g.Invites {
			add(inv.To, "group_invite", inv.From, id, day)
		}
		for _, u := range g.Requests {
			add(g.Creator, "group_join_request", u, id, 4*hour)
		}
		for _, m := range g.Members {
			if m.Via == "request" {
				add(m.User, "group_request_accepted", g.Creator, id, m.Ago)
			}
		}
		for i := range g.Events {
			for _, m := range g.Members {
				add(m.User, "group_event", g.Creator, s.events[fmt.Sprintf("%s/%d", g.Key, i)], day)
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].ago > rows[j].ago })
	for _, r := range rows {
		read := 0
		if r.ago > 2*day {
			read = 1
		}
		if _, err := s.tx.Exec(`INSERT INTO notifications (user_id, type, actor_id, ref_id, is_read, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			s.users[r.user], r.typ, s.users[r.actor], r.ref, read, s.at(-r.ago)); err != nil {
			return fmt.Errorf("notification %s for %s: %w", r.typ, r.user, err)
		}
	}
	return nil
}
