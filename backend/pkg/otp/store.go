package otp

import (
	"database/sql"
	"errors"
	"sync"
)

type memoryKey struct {
	purpose Purpose
	subject string
}

type MemoryStore struct {
	mu sync.Mutex
	m  map[memoryKey]Record
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{m: map[memoryKey]Record{}} }

func (s *MemoryStore) Get(p Purpose, subject string) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.m[memoryKey{p, subject}]; ok {
		return &r, nil
	}
	return nil, nil
}

func (s *MemoryStore) Set(p Purpose, subject string, r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[memoryKey{p, subject}] = r
	return nil
}

func (s *MemoryStore) Delete(p Purpose, subject string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, memoryKey{p, subject})
	return nil
}

type SQLStore struct {
	DB *sql.DB
}

func (s SQLStore) Get(p Purpose, subject string) (*Record, error) {
	var r Record
	err := s.DB.QueryRow(`SELECT code_hash, salt, expires_at, attempts, last_sent_at FROM otp_codes WHERE purpose = ? AND subject = ?`, p, subject).
		Scan(&r.CodeHash, &r.Salt, &r.ExpiresAt, &r.Attempts, &r.LastSentAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s SQLStore) Set(p Purpose, subject string, r Record) error {
	_, err := s.DB.Exec(`INSERT INTO otp_codes (purpose, subject, code_hash, salt, expires_at, attempts, last_sent_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(purpose, subject) DO UPDATE SET code_hash = excluded.code_hash, salt = excluded.salt,
		expires_at = excluded.expires_at, attempts = excluded.attempts, last_sent_at = excluded.last_sent_at`,
		p, subject, r.CodeHash, r.Salt, r.ExpiresAt.UTC(), r.Attempts, r.LastSentAt.UTC())
	return err
}

func (s SQLStore) Delete(p Purpose, subject string) error {
	_, err := s.DB.Exec(`DELETE FROM otp_codes WHERE purpose = ? AND subject = ?`, p, subject)
	return err
}
