package otp

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"social-network/pkg/db/sqlite"
	"social-network/pkg/mail"
)

type fakeSender struct {
	sent []mail.Message
	fail bool
}

func (f *fakeSender) Send(m mail.Message) error {
	if f.fail {
		return errors.New("smtp down")
	}
	f.sent = append(f.sent, m)
	return nil
}
func (f *fakeSender) FromName() string { return "Test App" }

var codeRe = regexp.MustCompile(`code is (\d+)`)

func codeOf(m mail.Message) string { return codeRe.FindStringSubmatch(m.Text)[1] }

func setup(t *testing.T, store Store) (*Service, *fakeSender, *time.Time) {
	clock := time.Unix(1_000_000, 0)
	sender := &fakeSender{}
	return New(store, sender, Options{Now: func() time.Time { return clock }}), sender, &clock
}

func stores(t *testing.T) map[string]Store {
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "otp.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.MigrateUp(db, "file://../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	return map[string]Store{"memory": NewMemoryStore(), "sql": SQLStore{DB: db}}
}

func TestGenerateCode(t *testing.T) {
	for i := 0; i < 200; i++ {
		if c := GenerateCode(6); !regexp.MustCompile(`^\d{6}$`).MatchString(c) {
			t.Fatalf("bad code %q", c)
		}
	}
	if len(GenerateCode(8)) != 8 {
		t.Fatal("length 8")
	}
}

func TestRightCodeWorksOnce(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			s, sender, _ := setup(t, store)
			if r, err := s.Send("u1", "a@x.com", ""); err != nil || !r.OK {
				t.Fatal(r, err)
			}
			code := codeOf(sender.sent[0])
			rec, _ := store.Get("u1")
			if strings.Contains(rec.CodeHash+rec.Salt, code) {
				t.Fatal("the code itself is stored")
			}
			if r, _ := s.Verify("u1", code); !r.OK {
				t.Fatal("right code refused")
			}
			if r, _ := s.Verify("u1", code); r.Reason != Missing {
				t.Fatal("code worked twice")
			}
		})
	}
}

func TestExpired(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			s, sender, clock := setup(t, store)
			s.Send("u1", "a@x.com", "")
			*clock = clock.Add(10*time.Minute + time.Second)
			if r, _ := s.Verify("u1", codeOf(sender.sent[0])); r.Reason != Expired {
				t.Fatal(r)
			}
		})
	}
}

func TestTooManyWrong(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			s, sender, _ := setup(t, store)
			s.Send("u1", "a@x.com", "")
			code := codeOf(sender.sent[0])
			wrong := "000000"
			if code == wrong {
				wrong = "111111"
			}
			for left := 4; left >= 1; left-- {
				if r, _ := s.Verify("u1", wrong); r.Reason != Wrong || r.AttemptsLeft != left {
					t.Fatal(r)
				}
			}
			if r, _ := s.Verify("u1", wrong); r.Reason != TooMany {
				t.Fatal(r)
			}
			if r, _ := s.Verify("u1", code); r.Reason != Missing {
				t.Fatal("code still usable after too many guesses")
			}
		})
	}
}

func TestCooldownAndFailedSend(t *testing.T) {
	for name, store := range stores(t) {
		t.Run(name, func(t *testing.T) {
			s, sender, clock := setup(t, store)
			s.Send("u1", "a@x.com", "")
			if r, _ := s.Send("u1", "a@x.com", ""); r.Reason != Cooldown || r.RetryAfter != 60 {
				t.Fatal(r)
			}
			*clock = clock.Add(time.Minute)
			if r, _ := s.Send("u1", "a@x.com", ""); !r.OK {
				t.Fatal(r)
			}
			if r, _ := s.Verify("u1", codeOf(sender.sent[1])); !r.OK {
				t.Fatal("new code refused")
			}
			sender.fail = true
			if _, err := s.Send("u2", "b@x.com", ""); err == nil {
				t.Fatal("no error")
			}
			if rec, _ := store.Get("u2"); rec != nil {
				t.Fatal("failed send kept a record (would block retry)")
			}
		})
	}
}
