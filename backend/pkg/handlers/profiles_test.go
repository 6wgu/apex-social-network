package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// register creates a user through the API and returns a logged-in client and the user's ID.
func register(t *testing.T, base, email, first string) (*testClient, int64) {
	t.Helper()
	c := newClient(t, base)
	fields := validUser(email)
	fields["first_name"] = first
	status, user := c.form("POST", "/api/register", fields, nil)
	if status != http.StatusCreated {
		t.Fatalf("register %s = %d %v", email, status, user)
	}
	return c, int64(user["id"].(float64))
}

func TestPrivateProfileIsLimitedInTheBackend(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()

	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")
	bob, bobID := register(t, srv.URL, "bob@example.com", "Bob")
	alicePath := fmt.Sprintf("/api/users/%d", aliceID)

	// Public: Bob sees everything.
	status, profile := bob.get(alicePath)
	user, _ := profile["user"].(map[string]any)
	if status != http.StatusOK || profile["can_view"] != true || user["email"] != "alice@example.com" {
		t.Fatalf("public profile seen by another user = %d %v", status, profile)
	}

	// Alice goes private.
	if status, body := alice.json("PATCH", "/api/me/privacy", `{"is_public": false}`); status != http.StatusOK || body["is_public"] != false {
		t.Fatalf("PATCH /api/me/privacy = %d %v", status, body)
	}

	// Bob now gets the limited shape and nothing else.
	status, profile = bob.get(alicePath)
	user, _ = profile["user"].(map[string]any)
	if status != http.StatusOK || profile["can_view"] != false || profile["follow_status"] != "none" {
		t.Fatalf("private profile seen by a stranger = %d %v", status, profile)
	}
	for _, secret := range []string{"email", "date_of_birth", "about_me", "created_at", "is_public"} {
		if _, leaked := user[secret]; leaked {
			t.Errorf("limited profile leaks %q: %v", secret, user)
		}
	}
	if user["first_name"] != "Alice" {
		t.Errorf("limited profile should still show the name, got %v", user)
	}
	if _, ok := profile["follower_count"]; ok {
		t.Errorf("limited profile leaks follower_count: %v", profile)
	}

	// Alice still sees her own full profile.
	status, profile = alice.get(alicePath)
	if status != http.StatusOK || profile["can_view"] != true || profile["follow_status"] != "self" {
		t.Errorf("own private profile = %d %v", status, profile)
	}

	// A pending request is not enough; an accepted one is.
	if _, err := app.DB.Exec(`INSERT INTO follows (follower_id, following_id, status) VALUES (?, ?, 'pending')`, bobID, aliceID); err != nil {
		t.Fatal(err)
	}
	if _, profile = bob.get(alicePath); profile["can_view"] != false || profile["follow_status"] != "pending" {
		t.Errorf("pending follower = %v, want can_view false / pending", profile)
	}
	if _, err := app.DB.Exec(`UPDATE follows SET status = 'accepted' WHERE follower_id = ? AND following_id = ?`, bobID, aliceID); err != nil {
		t.Fatal(err)
	}
	_, profile = bob.get(alicePath)
	user, _ = profile["user"].(map[string]any)
	if profile["can_view"] != true || user["email"] != "alice@example.com" || profile["follower_count"] != float64(1) {
		t.Errorf("accepted follower = %v, want the full profile", profile)
	}

	// Bob's privacy change only ever touches Bob.
	if status, body := bob.json("PATCH", "/api/me/privacy", `{"is_public": true}`); status != http.StatusOK || body["id"] != float64(bobID) {
		t.Errorf("bob's PATCH = %d %v", status, body)
	}
	var alicePublic bool
	app.DB.QueryRow(`SELECT is_public FROM users WHERE id = ?`, aliceID).Scan(&alicePublic)
	if alicePublic {
		t.Error("alice's profile became public after bob's request")
	}

	if status, _ := bob.get("/api/users/9999"); status != http.StatusNotFound {
		t.Errorf("unknown user = %d, want 404", status)
	}
}

func TestProfileRoutesNeedASession(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	c := newClient(t, srv.URL)

	for _, route := range [][2]string{
		{"GET", "/api/me"}, {"PATCH", "/api/me"}, {"PATCH", "/api/me/privacy"},
		{"GET", "/api/users"}, {"GET", "/api/users/1"}, {"POST", "/api/logout"},
	} {
		if status, body := c.json(route[0], route[1], `{}`); status != http.StatusUnauthorized || body["error"] == nil {
			t.Errorf("%s %s without a session = %d %v, want 401", route[0], route[1], status, body)
		}
	}
}

func TestUpdateMeAndSearch(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	alice, _ := register(t, srv.URL, "alice@example.com", "Alice")
	register(t, srv.URL, "bob@example.com", "Bob")

	status, user := alice.form("PATCH", "/api/me", map[string]string{"nickname": "al_100%", "about_me": "hello"}, tinyPNG(t))
	if status != http.StatusOK || user["nickname"] != "al_100%" || user["about_me"] != "hello" || user["avatar_path"] == "" {
		t.Fatalf("PATCH /api/me = %d %v", status, user)
	}
	if status, body := alice.form("PATCH", "/api/me", map[string]string{}, nil); status != http.StatusBadRequest {
		t.Errorf("empty PATCH /api/me = %d %v, want 400", status, body)
	}

	search := func(q string) []any {
		t.Helper()
		res, err := alice.http.Get(srv.URL + "/api/users?search=" + q)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var list []any
		if err := json.NewDecoder(res.Body).Decode(&list); err != nil || list == nil {
			t.Fatalf("search %q: not a JSON array (%v)", q, err)
		}
		return list
	}
	if got := search(""); len(got) != 2 {
		t.Errorf("empty search returned %d users, want 2", len(got))
	}
	if got := search("bob"); len(got) != 1 {
		t.Errorf("search bob returned %d users, want 1", len(got))
	}
	if got := search("al_100%25"); len(got) != 1 {
		t.Errorf("search by nickname returned %d users, want 1", len(got))
	}
	// % and _ are literal, not wildcards.
	if got := search("%25"); len(got) != 1 {
		t.Errorf("search %% returned %d users, want only the nickname containing %%", len(got))
	}
	if got := search("nobody"); len(got) != 0 {
		t.Errorf("search nobody returned %d users, want 0", len(got))
	}
}
