package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// registerWithNickname registers email with nickname and returns the client,
// the status and the body.
func registerWithNickname(t *testing.T, base, email, nickname string) (*testClient, int, map[string]any) {
	t.Helper()
	c := newClient(t, base)
	fields := validUser(email)
	fields["nickname"] = nickname
	status, body := c.form("POST", "/api/register", fields, nil)
	return c, status, body
}

// wantNicknameTaken checks a 409 in the field-error shape for the nickname.
func wantNicknameTaken(t *testing.T, what string, status int, body map[string]any) {
	t.Helper()
	fields, _ := body["fields"].(map[string]any)
	if status != http.StatusConflict || fields["nickname"] != nicknameTaken || body["error"] != nicknameTaken {
		t.Errorf("%s = %d %v, want 409 with fields.nickname %q", what, status, body, nicknameTaken)
	}
}

func TestNicknameIsUniqueOnRegister(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()

	if _, status, body := registerWithNickname(t, srv.URL, "ahmed@example.com", "Ahmed"); status != http.StatusCreated {
		t.Fatalf("first Ahmed = %d %v", status, body)
	}
	for _, nickname := range []string{"Ahmed", "ahmed", "AHMED", "  aHmEd  "} {
		_, status, body := registerWithNickname(t, srv.URL, "other@example.com", nickname)
		wantNicknameTaken(t, "register with nickname "+nickname, status, body)
	}

	// A taken email and a taken nickname come back together.
	_, status, body := registerWithNickname(t, srv.URL, "ahmed@example.com", "ahmed")
	fields, _ := body["fields"].(map[string]any)
	if status != http.StatusConflict || fields["email"] != "email already registered" || fields["nickname"] != nicknameTaken {
		t.Errorf("taken email and nickname = %d %v, want 409 with both fields", status, body)
	}

	// Nicknames are optional: any number of users can have none.
	for _, email := range []string{"none1@example.com", "none2@example.com", "none3@example.com"} {
		if _, status, body := registerWithNickname(t, srv.URL, email, ""); status != http.StatusCreated {
			t.Errorf("register %s without a nickname = %d %v, want 201", email, status, body)
		}
	}
}

func TestNicknameIsUniqueOnEdit(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()

	owner, status, body := registerWithNickname(t, srv.URL, "owner@example.com", "Ahmed")
	if status != http.StatusCreated {
		t.Fatalf("register owner = %d %v", status, body)
	}
	other, status, body := registerWithNickname(t, srv.URL, "other@example.com", "")
	if status != http.StatusCreated {
		t.Fatalf("register other = %d %v", status, body)
	}

	for _, nickname := range []string{"Ahmed", "ahmed", " AHMED "} {
		status, body := other.form("PATCH", "/api/me", map[string]string{"nickname": nickname}, nil)
		wantNicknameTaken(t, "edit to someone else's nickname "+nickname, status, body)
	}

	// My own nickname in another case, or with spaces, is still mine.
	for _, nickname := range []string{"AHMED", "  ahmed  ", "Ahmed"} {
		status, user := owner.form("PATCH", "/api/me", map[string]string{"nickname": nickname}, nil)
		if status != http.StatusOK {
			t.Errorf("edit my own nickname to %q = %d %v, want 200", nickname, status, user)
		}
	}

	// Freeing a nickname makes it available to others; clearing is always fine.
	if status, body := owner.form("PATCH", "/api/me", map[string]string{"nickname": ""}, nil); status != http.StatusOK {
		t.Fatalf("clear my nickname = %d %v", status, body)
	}
	if status, user := other.form("PATCH", "/api/me", map[string]string{"nickname": "ahmed"}, nil); status != http.StatusOK || user["nickname"] != "ahmed" {
		t.Errorf("take a freed nickname = %d %v, want 200", status, user)
	}

	// A bad format is a 400 in the same field shape.
	status, body = other.form("PATCH", "/api/me", map[string]string{"nickname": "1bad"}, nil)
	if fields, _ := body["fields"].(map[string]any); status != http.StatusBadRequest || fields["nickname"] == nil {
		t.Errorf("edit to a bad nickname = %d %v, want 400 with fields.nickname", status, body)
	}
}

func TestNicknameAvailability(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()

	owner, status, body := registerWithNickname(t, srv.URL, "owner@example.com", "Ahmed")
	if status != http.StatusCreated {
		t.Fatalf("register = %d %v", status, body)
	}
	anonymous := newClient(t, srv.URL)
	check := func(c *testClient, nickname string) (int, map[string]any) {
		return c.get("/api/nicknames/available?nickname=" + nickname)
	}

	for nickname, want := range map[string]bool{"Ahmed": false, "ahmed": false, "AHMED": false, "%20ahmed%20": false, "free_name": true, "Ahmed2": true} {
		status, body := check(anonymous, nickname)
		if status != http.StatusOK || body["available"] != want || len(body) != 1 {
			t.Errorf("available?nickname=%s = %d %v, want 200 {available: %v} and nothing else", nickname, status, body, want)
		}
	}
	for _, nickname := range []string{"", "1bad", "_bad", "bad.name", "%C3%A4da", "abcdefghijklmnopqrstuvwxyz12345"} {
		status, body := check(anonymous, nickname)
		if fields, _ := body["fields"].(map[string]any); status != http.StatusBadRequest || fields["nickname"] == nil {
			t.Errorf("available?nickname=%s = %d %v, want 400 with fields.nickname", nickname, status, body)
		}
	}

	// Signed in, my own nickname (in any case) is available to me.
	if status, body := check(owner, "AHMED"); status != http.StatusOK || body["available"] != true {
		t.Errorf("my own nickname for me = %d %v, want available", status, body)
	}
}

// Two registrations racing for one nickname both pass the check; the unique
// index stops the second INSERT, and uniqueConflict turns that into the
// nickname (or email) 409.
func TestUniqueConflictNamesTheField(t *testing.T) {
	app := newTestApp(t)
	insert := func(email, nickname string) error {
		_, err := app.DB.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, nickname)
			VALUES (?, 'h', 'A', 'B', '1990-01-01', ?)`, email, nickname)
		return err
	}
	if err := insert("a@example.com", "Racer"); err != nil {
		t.Fatal(err)
	}
	if field := uniqueConflict(insert("b@example.com", "racer")); field != "nickname" {
		t.Errorf("duplicate nickname: uniqueConflict = %q, want nickname", field)
	}
	if field := uniqueConflict(insert("A@example.com", "someone")); field != "email" {
		t.Errorf("duplicate email: uniqueConflict = %q, want email", field)
	}
	if field := uniqueConflict(insert("c@example.com", "")); field != "" {
		t.Errorf("no nickname: uniqueConflict = %q, want no conflict", field)
	}
}
