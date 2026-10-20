package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestValidEmail(t *testing.T) {
	valid := []string{
		"name@example.com",
		"123ahmed@gmail.com",
		"first.last+tag@sub.domain.org",
		"a_b%c-d@x-y.co",
		strings.Repeat("a", 64) + "@example.com",
	}
	for _, email := range valid {
		if !validEmail(email) {
			t.Errorf("validEmail(%q) = false, want true", email)
		}
	}

	invalid := []string{
		"",
		"a@b",        // no dot in the domain
		"a@b.",       // empty last label
		"@x.com",     // empty local part
		"a@.com",     // empty first label
		"a b@x.com",  // space
		"a@@x.com",   // two @
		"a@x@y.com",  // two @, apart
		"a@x..com",   // empty label between dots
		".a@x.com",   // local part starts with a dot
		"a.@x.com",   // local part ends with a dot
		"a..b@x.com", // two dots in a row
		"a@x.c",      // one-letter TLD
		"a@x.c0m",    // digit in the TLD
		"a@-x.com",   // label starts with a hyphen
		"a@x-.com",   // label ends with a hyphen
		"a@x_y.com",  // underscore in the domain
		"Ada <a@x.com>",
		"\"a\"@x.com",
		"a(comment)@x.com",
		"äda@x.com",
		strings.Repeat("a", 65) + "@example.com", // local part over 64
		"a@" + strings.Repeat("b", 64) + ".com",  // label over 63
		strings.Repeat("a", 61) + "@" + strings.Repeat("b.", 95) + "com", // 255 characters
	}
	for _, email := range invalid {
		if validEmail(email) {
			t.Errorf("validEmail(%q) = true, want false", email)
		}
	}

	if got := emailProblem(strings.Repeat("a", 64) + "@" + strings.Repeat("b", 186) + ".com"); !strings.Contains(got, "254 characters") {
		t.Errorf("255-character email: problem %q, want the length message", got)
	}
	if got := emailProblem(""); got != "email is required" {
		t.Errorf("empty email: problem %q", got)
	}
}

func TestPasswordProblem(t *testing.T) {
	cases := map[string]string{
		"Abcdef1!":                       "",
		"Correct-Horse1":                 "",
		"Pässwört1€":                     "", // non-ASCII letters and a currency symbol count
		"":                               "password is required",
		"Ab1!":                           "password needs at least 8 characters",
		"abcdefg1!":                      "password needs an uppercase letter",
		"ABCDEFG1!":                      "password needs a lowercase letter",
		"Abcdefgh!":                      "password needs a number",
		"Abcdefgh1":                      "password needs a special character",
		"abc":                            "password needs at least 8 characters, an uppercase letter, a number and a special character",
		"Aa1!" + strings.Repeat("x", 69): "password must be 72 bytes or fewer",
	}
	for password, want := range cases {
		if got := passwordProblem(password); got != want {
			t.Errorf("passwordProblem(%q) = %q, want %q", password, got, want)
		}
	}
}

func TestBirthDateLimits(t *testing.T) {
	day := func(s string) time.Time {
		d, err := time.Parse(dateLayout, s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	today := day("2026-10-20")
	date := func(years, days int) string { return today.AddDate(-years, 0, days).Format(dateLayout) }
	tooYoung := "you must be at least 7 years old"
	tooOld := "you must be 120 years old or younger"

	cases := []struct {
		dob, want string
	}{
		{date(MinAge, 0), ""},                                // exactly 7 today
		{date(MinAge, 1), tooYoung},                          // 7 tomorrow
		{date(MinAge+30, 0), ""},                             // an ordinary adult
		{date(MaxAge+1, 1), ""},                              // 121 tomorrow: still 120
		{date(MaxAge+1, 0), tooOld},                          // 121 today
		{date(MaxAge+5, 0), tooOld},                          // 125
		{date(0, 0), tooYoung},                               // born today
		{date(0, 1), "date of birth can't be in the future"}, // tomorrow
		{"2999-01-01", "date of birth can't be in the future"},
		{"", "date of birth is required"},
		{"2019-02-30", "date of birth must be a valid date (YYYY-MM-DD)"},
		{"20/10/2019", "date of birth must be a valid date (YYYY-MM-DD)"},
		{"2019-1-5", "date of birth must be a valid date (YYYY-MM-DD)"},
	}
	for _, c := range cases {
		if got := birthDateProblem(c.dob, today); got != c.want {
			t.Errorf("birthDateProblem(%q) on %s = %q, want %q", c.dob, today.Format(dateLayout), got, c.want)
		}
	}

	// The messages follow the constants.
	if !strings.Contains(tooYoung, " 7 ") || !strings.Contains(tooOld, " 120 ") || MinAge != 7 || MaxAge != 120 {
		t.Fatalf("update this test's messages for MinAge %d / MaxAge %d", MinAge, MaxAge)
	}

	// 29 February: a year older on 1 March in other years.
	leapDay := day("2028-02-29")
	if got := birthDateProblem("2021-02-28", leapDay); got != "" {
		t.Errorf("born 2021-02-28 on 2028-02-29: %q, want valid (7)", got)
	}
	if got := birthDateProblem("2021-03-01", leapDay); got != tooYoung {
		t.Errorf("born 2021-03-01 on 2028-02-29: %q, want too young (6)", got)
	}
	if age := ageOn(day("2020-02-29"), day("2027-02-28")); age != 6 {
		t.Errorf("born 2020-02-29, age on 2027-02-28 = %d, want 6", age)
	}
	if age := ageOn(day("2020-02-29"), day("2027-03-01")); age != 7 {
		t.Errorf("born 2020-02-29, age on 2027-03-01 = %d, want 7", age)
	}
}

func TestRegisterReportsEveryFieldError(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	c := newClient(t, srv.URL)

	young := time.Now().AddDate(-MinAge+1, 0, 0).Format(dateLayout)
	status, body := c.form("POST", "/api/register", map[string]string{
		"email": "a@b", "password": "weak", "first_name": "Ada", "last_name": "", "date_of_birth": young,
	}, nil)
	fields, _ := body["fields"].(map[string]any)
	if status != http.StatusBadRequest || len(fields) != 4 {
		t.Fatalf("register with four bad fields = %d %v, want 400 with four field errors", status, body)
	}
	want := map[string]string{
		"email":         "email must be a valid address like name@example.com",
		"password":      "password needs at least 8 characters, an uppercase letter, a number and a special character",
		"last_name":     "last name is required",
		"date_of_birth": "you must be at least 7 years old",
	}
	for field, msg := range want {
		if fields[field] != msg {
			t.Errorf("fields[%s] = %v, want %q", field, fields[field], msg)
		}
	}
	// "error" is the first problem in page order, for clients that only read it.
	if body["error"] != "last name is required" {
		t.Errorf("error = %v, want the last name problem", body["error"])
	}

	// Spaces and capitals around a valid email are cleaned up, not rejected.
	if status, user := c.form("POST", "/api/register", validUser("  123Ahmed@Gmail.COM  "), nil); status != http.StatusCreated || user["email"] != "123ahmed@gmail.com" {
		t.Fatalf("register with a padded mixed-case email = %d %v", status, user)
	}
	other := newClient(t, srv.URL)
	status, body = other.form("POST", "/api/register", validUser("123ahmed@gmail.com"), nil)
	fields, _ = body["fields"].(map[string]any)
	if status != http.StatusConflict || fields["email"] != "email already registered" {
		t.Errorf("duplicate email = %d %v, want 409 with fields.email", status, body)
	}
}

// Login never checks password strength: an account made before the rules
// existed still signs in with its old password.
func TestLoginAcceptsAnOldWeakPassword(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, is_verified)
		VALUES ('old@example.com', ?, 'Old', 'Timer', '1980-01-01', 1)`, string(hash)); err != nil {
		t.Fatal(err)
	}
	c := newClient(t, srv.URL)
	if status, body := c.json("POST", "/api/login", `{"email":"old@example.com","password":"oldpass"}`); status != http.StatusOK {
		t.Errorf("login with an old weak password = %d %v, want 200", status, body)
	}
}
