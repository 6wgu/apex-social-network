package otp

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"social-network/pkg/mail"
)

// fakeSender keeps the last code it was asked to send.
type fakeSender struct {
	code string
	fail bool
}

var sixDigits = regexp.MustCompile(`\d{6}`)

func (s *fakeSender) Send(m mail.Message) error {
	if s.fail {
		return errors.New("smtp down")
	}
	s.code = sixDigits.FindString(m.Subject)
	return nil
}

func (s *fakeSender) FromName() string { return "APEX" }

// newTestService returns a service on a memory store and a clock the test moves.
func newTestService() (*Service, *fakeSender, *time.Time) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	sender := &fakeSender{}
	svc := New(NewMemoryStore(), sender, Options{Now: func() time.Time { return now }})
	return svc, sender, &now
}

func TestGenerateCodeHasTheRightLength(t *testing.T) {
	for range 200 {
		if code := GenerateCode(6); !regexp.MustCompile(`^\d{6}$`).MatchString(code) {
			t.Fatalf("GenerateCode(6) = %q", code)
		}
	}
}

func TestSendThenVerify(t *testing.T) {
	svc, sender, _ := newTestService()
	if res, err := svc.Send("user:1", "a@example.com", "Ada"); err != nil || !res.OK {
		t.Fatalf("Send = %+v, %v", res, err)
	}
	if res, err := svc.Verify("user:1", " "+sender.code+" "); err != nil || !res.OK {
		t.Fatalf("Verify = %+v, %v", res, err)
	}
	// A code works once.
	if res, _ := svc.Verify("user:1", sender.code); res.Reason != Missing {
		t.Fatalf("second Verify reason = %q, want %q", res.Reason, Missing)
	}
}

func TestResendWaitsForTheCooldown(t *testing.T) {
	svc, _, now := newTestService()
	svc.Send("user:1", "a@example.com", "")
	*now = now.Add(20 * time.Second)
	res, err := svc.Send("user:1", "a@example.com", "")
	if err != nil || res.Reason != Cooldown || res.RetryAfter != 40 {
		t.Fatalf("Send during cooldown = %+v, %v; want cooldown with 40 s left", res, err)
	}
	*now = now.Add(40 * time.Second)
	if res, _ := svc.Send("user:1", "a@example.com", ""); !res.OK {
		t.Fatalf("Send after cooldown = %+v", res)
	}
}

func TestWrongCodesRunOut(t *testing.T) {
	svc, sender, _ := newTestService()
	svc.Send("user:1", "a@example.com", "")
	wrong := "000000"
	if sender.code == wrong {
		wrong = "111111"
	}
	for left := 4; left > 0; left-- {
		res, _ := svc.Verify("user:1", wrong)
		if res.Reason != Wrong || res.AttemptsLeft != left {
			t.Fatalf("Verify = %+v, want wrong with %d left", res, left)
		}
	}
	if res, _ := svc.Verify("user:1", wrong); res.Reason != TooMany {
		t.Fatalf("last Verify reason = %q, want %q", res.Reason, TooMany)
	}
	// The code is gone, so even the right one fails now.
	if res, _ := svc.Verify("user:1", sender.code); res.Reason != Missing {
		t.Fatalf("Verify after lockout reason = %q, want %q", res.Reason, Missing)
	}
}

func TestExpiredCode(t *testing.T) {
	svc, sender, now := newTestService()
	svc.Send("user:1", "a@example.com", "")
	*now = now.Add(11 * time.Minute)
	if res, _ := svc.Verify("user:1", sender.code); res.Reason != Expired {
		t.Fatalf("Verify reason = %q, want %q", res.Reason, Expired)
	}
}

func TestFailedSendLeavesNoCode(t *testing.T) {
	svc, sender, _ := newTestService()
	sender.fail = true
	if _, err := svc.Send("user:1", "a@example.com", ""); err == nil {
		t.Fatal("Send succeeded with a failing sender")
	}
	// No cooldown either: the user can try again straight away.
	sender.fail = false
	if res, _ := svc.Send("user:1", "a@example.com", ""); !res.OK {
		t.Fatalf("retry Send = %+v", res)
	}
}

func TestCancel(t *testing.T) {
	svc, sender, _ := newTestService()
	svc.Send("user:1", "a@example.com", "")
	svc.Cancel("user:1")
	if res, _ := svc.Verify("user:1", sender.code); res.Reason != Missing {
		t.Fatalf("Verify after Cancel reason = %q, want %q", res.Reason, Missing)
	}
}
