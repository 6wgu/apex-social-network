package otp

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"social-network/pkg/mail"
)

func GenerateCode(n int) string {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%0*s", n, v.String())
}

type Record struct {
	CodeHash   string
	Salt       string
	ExpiresAt  time.Time
	Attempts   int
	LastSentAt time.Time
}

type Store interface {
	Get(subject string) (*Record, error)
	Set(subject string, r Record) error
	Delete(subject string) error
}

type Sender interface {
	Send(mail.Message) error
	FromName() string
}

type Options struct {
	Length      int
	TTL         time.Duration
	MaxAttempts int
	ResendAfter time.Duration
	Now         func() time.Time
}

type Service struct {
	store  Store
	sender Sender
	opt    Options
}

func FromConfig(c mail.Config) Options {
	return Options{Length: c.OTPLength, TTL: time.Duration(c.OTPTTLMinutes) * time.Minute,
		MaxAttempts: c.OTPMaxAttempts, ResendAfter: time.Duration(c.OTPResendSeconds) * time.Second}
}

func New(store Store, sender Sender, opt Options) *Service {
	if opt.Length == 0 {
		opt.Length = 6
	}
	if opt.TTL == 0 {
		opt.TTL = 10 * time.Minute
	}
	if opt.MaxAttempts == 0 {
		opt.MaxAttempts = 5
	}
	if opt.ResendAfter == 0 {
		opt.ResendAfter = time.Minute
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	return &Service{store: store, sender: sender, opt: opt}
}

const (
	Cooldown = "cooldown"
	Missing  = "missing"
	Expired  = "expired"
	TooMany  = "too_many"
	Wrong    = "wrong"
)

type Result struct {
	OK           bool
	Reason       string
	RetryAfter   int
	AttemptsLeft int
}

func hash(code, salt string) string {
	sum := sha256.Sum256([]byte(salt + code))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Send(subject, email, name string) (Result, error) {
	now := s.opt.Now()
	old, err := s.store.Get(subject)
	if err != nil {
		return Result{}, err
	}
	if old != nil {
		if wait := old.LastSentAt.Add(s.opt.ResendAfter).Sub(now); wait > 0 {
			return Result{Reason: Cooldown, RetryAfter: int((wait + time.Second - 1) / time.Second)}, nil
		}
	}
	code := GenerateCode(s.opt.Length)
	saltBytes := make([]byte, 16)
	rand.Read(saltBytes)
	salt := hex.EncodeToString(saltBytes)
	rec := Record{CodeHash: hash(code, salt), Salt: salt, ExpiresAt: now.Add(s.opt.TTL), LastSentAt: now}
	if err := s.store.Set(subject, rec); err != nil {
		return Result{}, err
	}
	msg := mail.CodeEmail(s.sender.FromName(), name, code, int(s.opt.TTL/time.Minute))
	msg.To = []string{email}
	if err := s.sender.Send(msg); err != nil {
		s.store.Delete(subject)
		return Result{}, err
	}
	return Result{OK: true}, nil
}

func (s *Service) Verify(subject, code string) (Result, error) {
	rec, err := s.store.Get(subject)
	if err != nil || rec == nil {
		return Result{Reason: Missing}, err
	}
	if s.opt.Now().After(rec.ExpiresAt) {
		return Result{Reason: Expired}, s.store.Delete(subject)
	}
	if rec.Attempts >= s.opt.MaxAttempts {
		return Result{Reason: TooMany}, s.store.Delete(subject)
	}
	if subtle.ConstantTimeCompare([]byte(hash(strings.TrimSpace(code), rec.Salt)), []byte(rec.CodeHash)) == 1 {
		return Result{OK: true}, s.store.Delete(subject)
	}
	rec.Attempts++
	if rec.Attempts >= s.opt.MaxAttempts {
		return Result{Reason: TooMany}, s.store.Delete(subject)
	}
	return Result{Reason: Wrong, AttemptsLeft: s.opt.MaxAttempts - rec.Attempts}, s.store.Set(subject, *rec)
}

func (s *Service) Cancel(subject string) error {
	return s.store.Delete(subject)
}
