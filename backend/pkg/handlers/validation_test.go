package handlers

import (
	"strings"
	"testing"
	"time"
)

// The same cases are checked against frontend/lib/validation.js in
// frontend/lib/validation.test.mjs; keep the two in step.

func TestValidEmail(t *testing.T) {
	good := []string{
		"name@example.com",
		"123ahmed@gmail.com",
		"a.b_c%d+e-f@sub.example.co",
		strings.Repeat("a", 64) + "@example.com",
	}
	bad := []string{
		"",
		"plain",
		"a@b",
		"a@@example.com",
		"a@b@example.com",
		".a@example.com",
		"a.@example.com",
		"a..b@example.com",
		strings.Repeat("a", 65) + "@example.com",
		"a@-example.com",
		"a@example-.com",
		"a@example..com",
		"a@example.c",
		"a@example.c0m",
		"a b@example.com",
		"ä@example.com",
		"Ada <a@example.com>",
	}
	for _, e := range good {
		if !validEmail(e) {
			t.Errorf("validEmail(%q) = false, want true", e)
		}
	}
	for _, e := range bad {
		if validEmail(e) {
			t.Errorf("validEmail(%q) = true, want false", e)
		}
	}
}

func TestPasswordProblem(t *testing.T) {
	cases := map[string]string{
		"":                         "password is required",
		strongPassword:             "",
		"short1!A":                 "",
		"alllowercase":             "password needs an uppercase letter, a number and a special character",
		"Ab1!":                     "password needs at least 8 characters",
		"NoSpecial123":             "password needs a special character",
		strings.Repeat("Aa1!", 19): "password must be 72 bytes or fewer",
	}
	for pw, want := range cases {
		if got := passwordProblem(pw); got != want {
			t.Errorf("passwordProblem(%q) = %q, want %q", pw, got, want)
		}
	}
}

func TestBirthDateProblem(t *testing.T) {
	now := time.Date(2026, 3, 1, 15, 0, 0, 0, time.UTC)
	cases := map[string]string{
		"":           "date of birth is required",
		"2000-02-30": "date of birth must be a valid date (YYYY-MM-DD)",
		"01/01/2000": "date of birth must be a valid date (YYYY-MM-DD)",
		"2026-03-02": "date of birth can't be in the future",
		"2019-03-01": "", // turns 7 today
		"2019-03-02": "you must be at least 7 years old",
		"1906-03-02": "", // 119 until tomorrow
		"1905-03-01": "you must be 120 years old or younger",
	}
	for dob, want := range cases {
		if got := birthDateProblem(dob, now); got != want {
			t.Errorf("birthDateProblem(%q) = %q, want %q", dob, got, want)
		}
	}
}

func TestAgeOnLeapDay(t *testing.T) {
	born := time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC)
	if got := ageOn(born, time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC)); got != 24 {
		t.Errorf("age on 28 Feb 2025 = %d, want 24", got)
	}
	if got := ageOn(born, time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)); got != 25 {
		t.Errorf("age on 1 Mar 2025 = %d, want 25", got)
	}
}

func TestValidateNickname(t *testing.T) {
	const badChars = "nickname must start with a letter and contain only letters, numbers, underscores, or hyphens"
	cases := map[string]string{
		"":                      "",
		"ada":                   "",
		"Ada_L-1":               "",
		"1ada":                  badChars,
		"_ada":                  badChars,
		"ada lovelace":          badChars,
		"adä":                   badChars,
		strings.Repeat("a", 30): "",
		strings.Repeat("a", 31): "nickname must be 30 characters or fewer",
	}
	for nick, want := range cases {
		if got := validateNickname(nick); got != want {
			t.Errorf("validateNickname(%q) = %q, want %q", nick, got, want)
		}
	}
}

func TestValidateAbout(t *testing.T) {
	if got := validateAbout(strings.Repeat("é", 500)); got != "" {
		t.Errorf("500 characters: %q, want none", got)
	}
	if got := validateAbout(strings.Repeat("é", 501)); got != "about me must be 500 characters or fewer" {
		t.Errorf("501 characters: %q", got)
	}
}

func TestJoinWithAnd(t *testing.T) {
	for in, want := range map[string]string{"a": "a", "a|b": "a and b", "a|b|c": "a, b and c"} {
		if got := joinWithAnd(strings.Split(in, "|")); got != want {
			t.Errorf("joinWithAnd(%q) = %q, want %q", in, got, want)
		}
	}
}
