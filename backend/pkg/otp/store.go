package otp

import (
	"database/sql"
	"errors"
	"sync"
)

type MemoryStore struct {
	mu sync.Mutex
	m  map[string]Record
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{m: map[string]Record{}} }

func (s *MemoryStore) Get(subject string) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.m[subject]; ok {
		return &r, nil
	}
	return nil, nil
}

func (s *MemoryStore) Set(subject string, r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[subject] = r
	return nil
}

func (s *MemoryStore) Delete(subject string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, subject)
	return nil
}

type SQLStore struct {
	DB *sql.DB
}

func (s SQLStore) Get(subject string) (*Record, error) {
	var r Record
	err := s.DB.QueryRow(`SELECT code_hash, salt, expires_at, attempts, last_sent_at FROM otp_codes WHERE subject = ?`, subject).
		Scan(&r.CodeHash, &r.Salt, &r.ExpiresAt, &r.Attempts, &r.LastSentAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s SQLStore) Set(subject string, r Record) error {
	_, err := s.DB.Exec(`INSERT INTO otp_codes (subject, code_hash, salt, expires_at, attempts, last_sent_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(subject) DO UPDATE SET code_hash = excluded.code_hash, salt = excluded.salt,
		expires_at = excluded.expires_at, attempts = excluded.attempts, last_sent_at = excluded.last_sent_at`,
		subject, r.CodeHash, r.Salt, r.ExpiresAt.UTC(), r.Attempts, r.LastSentAt.UTC())
	return err
}

func (s SQLStore) Delete(subject string) error {
	_, err := s.DB.Exec(`DELETE FROM otp_codes WHERE subject = ?`, subject)
	return err
}
