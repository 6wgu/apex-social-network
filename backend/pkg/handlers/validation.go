package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Validation of the register form and the profile fields. The frontend
// checks the same rules (frontend/lib/validation.js) so the user sees a
// problem at once; these are the ones that count, because a request can
// skip the page.

// Age limits for the date of birth, in whole years. A birthday not reached
// yet this year does not count. Change them here and MIN_AGE / MAX_AGE in
// frontend/lib/validation.js together.
const (
	MinAge = 7
	MaxAge = 120
)

const (
	maxEmailLen    = 254 // the longest address SMTP can carry
	maxEmailLocal  = 64  // characters before the "@"
	maxEmailLabel  = 63  // characters between two dots of the domain
	minPasswordLen = 8
	maxPasswordLen = 72 // bcrypt ignores everything after 72 bytes
	maxNameLen     = 50
	maxNicknameLen = 30
	maxAboutLen    = 500
	dateLayout     = "2006-01-02"
)

// validEmail reports whether email is a plain address like
// name@example.com. The caller trims and lowercases it first. net/mail
// alone accepts too much (display names, quoted local parts, a domain
// without a dot), so the rules are spelled out here:
//   - at most 254 characters, and exactly one "@"
//   - the local part (before the "@") is 1 to 64 characters of letters,
//     digits and . _ % + -, with no dot at its start or end and no two dots
//     in a row. It may start with a digit: 123ahmed@gmail.com is valid.
//   - the domain is two or more labels joined by single dots; each label is
//     1 to 63 letters, digits or hyphens and does not start or end with a
//     hyphen
//   - the last label (the TLD) is at least 2 letters and letters only
func validEmail(email string) bool {
	if utf8.RuneCountInString(email) > maxEmailLen {
		return false
	}
	local, domain, found := strings.Cut(email, "@")
	if !found || strings.Contains(domain, "@") {
		return false
	}
	if local == "" || len(local) > maxEmailLocal ||
		strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") || strings.Contains(local, "..") {
		return false
	}
	for _, c := range local {
		if !isASCIILetter(c) && !isASCIIDigit(c) && !strings.ContainsRune("._%+-", c) {
			return false
		}
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > maxEmailLabel ||
			strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, c := range label {
			if !isASCIILetter(c) && !isASCIIDigit(c) && c != '-' {
				return false
			}
		}
	}
	tld := labels[len(labels)-1]
	if len(tld) < 2 {
		return false
	}
	for _, c := range tld {
		if !isASCIILetter(c) {
			return false
		}
	}
	return true
}

func isASCIILetter(c rune) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }
func isASCIIDigit(c rune) bool  { return c >= '0' && c <= '9' }

// emailProblem returns what is wrong with a trimmed, lowercased email, or "".
func emailProblem(email string) string {
	switch {
	case email == "":
		return "email is required"
	case utf8.RuneCountInString(email) > maxEmailLen:
		return fmt.Sprintf("email must be %d characters or fewer", maxEmailLen)
	case !validEmail(email):
		return "email must be a valid address like name@example.com"
	}
	return ""
}

// passwordRules are the strength rules for a new password, in the order a
// 400 lists the missing ones. Only register checks them: login never does,
// so accounts made before the rules existed can still sign in.
var passwordRules = []struct {
	missing string
	met     func(string) bool
}{
	{fmt.Sprintf("at least %d characters", minPasswordLen), func(p string) bool { return utf8.RuneCountInString(p) >= minPasswordLen }},
	{"an uppercase letter", containsRune(unicode.IsUpper)},
	{"a lowercase letter", containsRune(unicode.IsLower)},
	{"a number", containsRune(unicode.IsDigit)},
	// Punctuation and symbols: ! @ # $ % ^ & * - _ + = ? and the like.
	{"a special character", containsRune(func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) })},
}

func containsRune(match func(rune) bool) func(string) bool {
	return func(s string) bool { return strings.IndexFunc(s, match) >= 0 }
}

// passwordProblem returns what a new password is missing, or "". Every
// missing rule is named, e.g. "password needs an uppercase letter and a
// number".
func passwordProblem(password string) string {
	if password == "" {
		return "password is required"
	}
	var missing []string
	for _, rule := range passwordRules {
		if !rule.met(password) {
			missing = append(missing, rule.missing)
		}
	}
	if len(missing) > 0 {
		return "password needs " + joinWithAnd(missing)
	}
	if len(password) > maxPasswordLen {
		return fmt.Sprintf("password must be %d bytes or fewer", maxPasswordLen)
	}
	return ""
}

// joinWithAnd turns ["a", "b", "c"] into "a, b and c".
func joinWithAnd(items []string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// birthDateProblem checks a YYYY-MM-DD date of birth on the calendar day of
// now: a real date, not in the future, and an age from MinAge to MaxAge.
func birthDateProblem(dob string, now time.Time) string {
	if dob == "" {
		return "date of birth is required"
	}
	born, err := time.Parse(dateLayout, dob)
	if err != nil {
		return "date of birth must be a valid date (YYYY-MM-DD)"
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch age := ageOn(born, today); {
	case born.After(today):
		return "date of birth can't be in the future"
	case age < MinAge:
		return fmt.Sprintf("you must be at least %d years old", MinAge)
	case age > MaxAge:
		return fmt.Sprintf("you must be %d years old or younger", MaxAge)
	}
	return ""
}

// ageOn is the age in whole years on the day on of someone born on born:
// the difference in years, minus one while that year's birthday has not
// come yet. Someone born on 29 February gets a year older on 1 March in
// other years.
func ageOn(born, on time.Time) int {
	age := on.Year() - born.Year()
	if on.Month() < born.Month() || (on.Month() == born.Month() && on.Day() < born.Day()) {
		age--
	}
	return age
}

// nameProblem checks a required name field; label is how messages call it.
func nameProblem(name, label string) string {
	switch {
	case name == "":
		return label + " is required"
	case utf8.RuneCountInString(name) > maxNameLen:
		return fmt.Sprintf("%s must be %d characters or fewer", label, maxNameLen)
	}
	return ""
}

// validateNickname and validateAbout are shared by register and edit profile.
func validateNickname(s string) string {
	if utf8.RuneCountInString(s) > maxNicknameLen {
		return "nickname must be 30 characters or fewer"
	}
	if s == "" {
		return ""
	}
	if !isNicknameLetter(s[0]) {
		return "nickname must start with a letter and contain only letters, numbers, underscores, or hyphens"
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !isNicknameLetter(c) && (c < '0' || c > '9') && c != '_' && c != '-' {
			return "nickname must start with a letter and contain only letters, numbers, underscores, or hyphens"
		}
	}
	return ""
}

func isNicknameLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func validateAbout(s string) string {
	if utf8.RuneCountInString(s) > maxAboutLen {
		return "about me must be 500 characters or fewer"
	}
	return ""
}

// registration is the text of a register form: trimmed, with the email
// lowercased. The password is kept exactly as typed.
type registration struct {
	Email, Password, FirstName, LastName, DateOfBirth, Nickname, AboutMe string
}

func readRegistration(r *http.Request) registration {
	field := func(name string) string { return strings.TrimSpace(r.FormValue(name)) }
	return registration{
		Email:       strings.ToLower(field("email")),
		Password:    r.FormValue("password"),
		FirstName:   field("first_name"),
		LastName:    field("last_name"),
		DateOfBirth: field("date_of_birth"),
		Nickname:    field("nickname"),
		AboutMe:     field("about_me"),
	}
}

// fieldErrors maps a form field to what is wrong with it.
type fieldErrors map[string]string

// registerFieldOrder is the order fields appear on the register page; the
// first field with a problem gives the response its "error" text.
var registerFieldOrder = []string{"first_name", "last_name", "email", "password", "date_of_birth", "nickname", "about_me", "avatar"}

// problems checks every field of reg on the day of now and returns all
// the problems at once (empty when the form is fine).
func (reg registration) problems(now time.Time) fieldErrors {
	errs := fieldErrors{}
	for field, problem := range map[string]string{
		"first_name":    nameProblem(reg.FirstName, "first name"),
		"last_name":     nameProblem(reg.LastName, "last name"),
		"email":         emailProblem(reg.Email),
		"password":      passwordProblem(reg.Password),
		"date_of_birth": birthDateProblem(reg.DateOfBirth, now),
		"nickname":      validateNickname(reg.Nickname),
		"about_me":      validateAbout(reg.AboutMe),
	} {
		if problem != "" {
			errs[field] = problem
		}
	}
	return errs
}

// writeFieldErrors answers with status and
// {"error": <first problem>, "fields": {"<field>": "<problem>", ...}}, so a
// client that only reads "error" still gets a message.
func writeFieldErrors(w http.ResponseWriter, status int, errs fieldErrors) {
	first := ""
	for _, field := range registerFieldOrder {
		if errs[field] != "" {
			first = errs[field]
			break
		}
	}
	writeJSON(w, status, map[string]any{"error": first, "fields": errs})
}
