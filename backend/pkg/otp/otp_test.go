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
	sent int
	fail bool
}

var sixDigits = regexp.MustCompile(`\d{6}`)

func (s *fakeSender) Send(m mail.Message) error {
	if s.fail {
		return errors.New("smtp down")
	}
	s.code = sixDigits.FindString(m.Text)
	s.sent++
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
	if res, err := svc.Send(VerifyEmail, "user:1", "a@example.com", "Ada"); err != nil || !res.OK {
		t.Fatalf("Send = %+v, %v", res, err)
	}
	if res, err := svc.Verify(VerifyEmail, "user:1", " "+sender.code+" "); err != nil || !res.OK {
		t.Fatalf("Verify = %+v, %v", res, err)
	}
	// A code works once.
	if res, _ := svc.Verify(VerifyEmail, "user:1", sender.code); res.Reason != Missing {
		t.Fatalf("second Verify reason = %q, want %q", res.Reason, Missing)
	}
}

func TestResendWaitsForTheCooldown(t *testing.T) {
	svc, _, now := newTestService()
	svc.Send(VerifyEmail, "user:1", "a@example.com", "")
	*now = now.Add(20 * time.Second)
	res, err := svc.Send(VerifyEmail, "user:1", "a@example.com", "")
	if err != nil || res.Reason != Cooldown || res.RetryAfter != 40 {
		t.Fatalf("Send during cooldown = %+v, %v; want cooldown with 40 s left", res, err)
	}
	*now = now.Add(40 * time.Second)
	if res, _ := svc.Send(VerifyEmail, "user:1", "a@example.com", ""); !res.OK {
		t.Fatalf("Send after cooldown = %+v", res)
	}
}

func TestWrongCodesRunOut(t *testing.T) {
	svc, sender, _ := newTestService()
	svc.Send(VerifyEmail, "user:1", "a@example.com", "")
	wrong := "000000"
	if sender.code == wrong {
		wrong = "111111"
	}
	for left := 4; left > 0; left-- {
		res, _ := svc.Verify(VerifyEmail, "user:1", wrong)
		if res.Reason != Wrong || res.AttemptsLeft != left {
			t.Fatalf("Verify = %+v, want wrong with %d left", res, left)
		}
	}
	if res, _ := svc.Verify(VerifyEmail, "user:1", wrong); res.Reason != TooMany {
		t.Fatalf("last Verify reason = %q, want %q", res.Reason, TooMany)
	}
	// The code is gone, so even the right one fails now.
	if res, _ := svc.Verify(VerifyEmail, "user:1", sender.code); res.Reason != Missing {
		t.Fatalf("Verify after lockout reason = %q, want %q", res.Reason, Missing)
	}
}

func TestExpiredCode(t *testing.T) {
	svc, sender, now := newTestService()
	svc.Send(VerifyEmail, "user:1", "a@example.com", "")
	*now = now.Add(11 * time.Minute)
	if res, _ := svc.Verify(VerifyEmail, "user:1", sender.code); res.Reason != Expired {
		t.Fatalf("Verify reason = %q, want %q", res.Reason, Expired)
	}
}

func TestFailedSendLeavesNoCode(t *testing.T) {
	svc, sender, _ := newTestService()
	sender.fail = true
	if _, err := svc.Send(VerifyEmail, "user:1", "a@example.com", ""); err == nil {
		t.Fatal("Send succeeded with a failing sender")
	}
	// No cooldown either: the user can try again straight away.
	sender.fail = false
	if res, _ := svc.Send(VerifyEmail, "user:1", "a@example.com", ""); !res.OK {
		t.Fatalf("retry Send = %+v", res)
	}
}

func TestCancel(t *testing.T) {
	svc, sender, _ := newTestService()
	svc.Send(VerifyEmail, "user:1", "a@example.com", "")
	svc.Cancel(VerifyEmail, "user:1")
	if res, _ := svc.Verify(VerifyEmail, "user:1", sender.code); res.Reason != Missing {
		t.Fatalf("Verify after Cancel reason = %q, want %q", res.Reason, Missing)
	}
}

// A code is only good for the purpose it was made for, and each purpose has
// its own code and cooldown for the same subject.
func TestPurposesAreSeparate(t *testing.T) {
	svc, sender, _ := newTestService()
	svc.Send(VerifyEmail, "user:1", "a@example.com", "")
	verifyCode := sender.code
	if res, _ := svc.Send(ResetPassword, "user:1", "a@example.com", ""); !res.OK {
		t.Fatalf("reset Send during the verification cooldown = %+v, want a code", res)
	}
	resetCode := sender.code
	if verifyCode == resetCode {
		t.Skip("the two random codes happen to be equal")
	}
	if res, _ := svc.Verify(ResetPassword, "user:1", verifyCode); res.Reason != Wrong {
		t.Fatalf("verification code as a reset code: reason %q, want %q", res.Reason, Wrong)
	}
	if res, _ := svc.Verify(VerifyEmail, "user:1", resetCode); res.Reason != Wrong {
		t.Fatalf("reset code as a verification code: reason %q, want %q", res.Reason, Wrong)
	}
	if res, _ := svc.Verify(VerifyEmail, "user:1", verifyCode); !res.OK {
		t.Fatalf("verification code = %+v, want OK", res)
	}
	if res, _ := svc.Verify(ResetPassword, "user:1", resetCode); !res.OK {
		t.Fatalf("reset code = %+v, want OK", res)
	}
}

func TestIssueStoresWithoutSending(t *testing.T) {
	svc, sender, _ := newTestService()
	code, res, err := svc.Issue(ResetPassword, "email:a@example.com")
	if err != nil || !res.OK || len(code) != 6 {
		t.Fatalf("Issue = %q, %+v, %v", code, res, err)
	}
	if sender.sent != 0 {
		t.Fatalf("Issue sent %d emails, want none", sender.sent)
	}
	if _, res, _ := svc.Issue(ResetPassword, "email:a@example.com"); res.Reason != Cooldown {
		t.Fatalf("second Issue reason = %q, want %q", res.Reason, Cooldown)
	}
	if err := svc.Deliver(ResetPassword, code, "a@example.com", "Ada"); err != nil || sender.code != code {
		t.Fatalf("Deliver sent %q (%v), want %q", sender.code, err, code)
	}
	if res, _ := svc.Verify(ResetPassword, "email:a@example.com", code); !res.OK {
		t.Fatalf("Verify = %+v, want OK", res)
	}
}
