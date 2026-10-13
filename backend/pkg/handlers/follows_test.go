package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

// list GETs a path that returns a JSON array and returns the status and items.
func (c *testClient) list(path string) (int, []map[string]any) {
	c.t.Helper()
	res, err := c.http.Get(c.base + path)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	var items []map[string]any
	json.NewDecoder(res.Body).Decode(&items)
	return res.StatusCode, items
}

// ids returns the sorted "id" (or "user.id") values of list items.
func ids(items []map[string]any) []int64 {
	out := []int64{}
	for _, it := range items {
		if u, ok := it["user"].(map[string]any); ok {
			it = u
		}
		out = append(out, int64(it["id"].(float64)))
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func followStatusOf(t *testing.T, c *testClient, id int64) string {
	t.Helper()
	_, p := c.get(fmt.Sprintf("/api/users/%d", id))
	s, _ := p["follow_status"].(string)
	return s
}

func TestFollowPublicIsInstant(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")
	bob, bobID := register(t, srv.URL, "bob@example.com", "Bob")
	path := fmt.Sprintf("/api/users/%d/follow", aliceID)

	if s := followStatusOf(t, bob, aliceID); s != "none" {
		t.Errorf("before following: %q, want none", s)
	}
	if status, body := bob.do("POST", path, "", nil); status != http.StatusOK || body["follow_status"] != "following" {
		t.Fatalf("follow public user = %d %v, want 200 following", status, body)
	}
	if s := followStatusOf(t, bob, aliceID); s != "following" {
		t.Errorf("after following: %q, want following", s)
	}
	if s := followStatusOf(t, alice, aliceID); s != "self" {
		t.Errorf("own profile: %q, want self", s)
	}
	if status, body := bob.do("POST", path, "", nil); status != http.StatusConflict {
		t.Errorf("follow twice = %d %v, want 409", status, body)
	}

	_, p := alice.get(fmt.Sprintf("/api/users/%d", aliceID))
	if p["follower_count"] != float64(1) || p["following_count"] != float64(0) {
		t.Errorf("alice counts = %v", p)
	}
	_, p = bob.get(fmt.Sprintf("/api/users/%d", bobID))
	if p["following_count"] != float64(1) {
		t.Errorf("bob following_count = %v", p["following_count"])
	}
	if _, list := bob.list(fmt.Sprintf("/api/users/%d/followers", aliceID)); fmt.Sprint(ids(list)) != fmt.Sprint([]int64{bobID}) {
		t.Errorf("alice followers = %v, want [bob]", list)
	}
	if _, list := alice.list(fmt.Sprintf("/api/users/%d/following", bobID)); fmt.Sprint(ids(list)) != fmt.Sprint([]int64{aliceID}) {
		t.Errorf("bob following = %v, want [alice]", list)
	}

	// Unfollow, then a clear 404 for unfollowing again.
	if status, _ := bob.do("DELETE", path, "", nil); status != http.StatusNoContent {
		t.Errorf("unfollow = %d, want 204", status)
	}
	if status, body := bob.do("DELETE", path, "", nil); status != http.StatusNotFound || body["error"] == nil {
		t.Errorf("unfollow when not following = %d %v, want 404 with a message", status, body)
	}
	if s := followStatusOf(t, bob, aliceID); s != "none" {
		t.Errorf("after unfollow: %q, want none", s)
	}
}

func TestFollowPrivateNeedsAcceptance(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")
	bob, bobID := register(t, srv.URL, "bob@example.com", "Bob")
	carol, _ := register(t, srv.URL, "carol@example.com", "Carol")
	alice.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	follow := fmt.Sprintf("/api/users/%d/follow", aliceID)

	if status, body := bob.do("POST", follow, "", nil); status != http.StatusOK || body["follow_status"] != "pending" {
		t.Fatalf("follow private user = %d %v, want 200 pending", status, body)
	}
	if status, body := bob.do("POST", follow, "", nil); status != http.StatusConflict {
		t.Errorf("duplicate request = %d %v, want 409", status, body)
	}
	if s := followStatusOf(t, bob, aliceID); s != "pending" {
		t.Errorf("bob -> alice = %q, want pending", s)
	}
	// Pending is not following: profile still limited, lists still hidden.
	if _, p := bob.get(fmt.Sprintf("/api/users/%d", aliceID)); p["can_view"] != false {
		t.Errorf("pending follower sees the private profile: %v", p)
	}
	if status, _ := bob.list(fmt.Sprintf("/api/users/%d/followers", aliceID)); status != http.StatusForbidden {
		t.Errorf("pending follower lists followers = %d, want 403", status)
	}

	// Alice sees the request; Bob and Carol cannot answer it.
	if _, reqs := alice.list("/api/follow-requests"); fmt.Sprint(ids(reqs)) != fmt.Sprint([]int64{bobID}) {
		t.Errorf("alice's requests = %v, want [bob]", reqs)
	}
	if _, reqs := bob.list("/api/follow-requests"); len(reqs) != 0 {
		t.Errorf("bob sees requests addressed to alice: %v", reqs)
	}
	accept := fmt.Sprintf("/api/follow-requests/%d/accept", bobID)
	if status, _ := carol.do("POST", accept, "", nil); status != http.StatusNotFound {
		t.Errorf("carol accepts a request sent to alice = %d, want 404", status)
	}
	if status, _ := bob.do("POST", fmt.Sprintf("/api/follow-requests/%d/accept", bobID), "", nil); status != http.StatusNotFound {
		t.Errorf("bob accepts his own request = %d, want 404", status)
	}
	if s := followStatusOf(t, bob, aliceID); s != "pending" {
		t.Fatalf("after forged accepts: %q, want pending", s)
	}

	// Decline: the row is gone, and Bob may ask again.
	if status, _ := alice.do("POST", fmt.Sprintf("/api/follow-requests/%d/decline", bobID), "", nil); status != http.StatusNoContent {
		t.Fatalf("decline = %d, want 204", status)
	}
	if status, _ := alice.do("POST", fmt.Sprintf("/api/follow-requests/%d/decline", bobID), "", nil); status != http.StatusNotFound {
		t.Errorf("decline twice = %d, want 404", status)
	}
	if s := followStatusOf(t, bob, aliceID); s != "none" {
		t.Errorf("after decline: %q, want none", s)
	}
	if status, body := bob.do("POST", follow, "", nil); status != http.StatusOK || body["follow_status"] != "pending" {
		t.Fatalf("request again after decline = %d %v", status, body)
	}

	// Accept: Bob is a follower and sees everything.
	if status, _ := alice.do("POST", accept, "", nil); status != http.StatusNoContent {
		t.Fatalf("accept = %d, want 204", status)
	}
	if s := followStatusOf(t, bob, aliceID); s != "following" {
		t.Errorf("after accept: %q, want following", s)
	}
	if _, p := bob.get(fmt.Sprintf("/api/users/%d", aliceID)); p["can_view"] != true || p["follower_count"] != float64(1) {
		t.Errorf("accepted follower profile = %v", p)
	}
	if status, list := bob.list(fmt.Sprintf("/api/users/%d/followers", aliceID)); status != http.StatusOK || len(list) != 1 {
		t.Errorf("follower lists followers = %d %v", status, list)
	}
	if status, _ := carol.list(fmt.Sprintf("/api/users/%d/following", aliceID)); status != http.StatusForbidden {
		t.Errorf("stranger lists a private user's following = %d, want 403", status)
	}
	if status, _ := alice.list(fmt.Sprintf("/api/users/%d/followers", aliceID)); status != http.StatusOK {
		t.Errorf("owner lists own followers = %d, want 200", status)
	}

	// Cancel a pending request (Carol), via the same DELETE as unfollow.
	carol.do("POST", follow, "", nil)
	if status, _ := carol.do("DELETE", follow, "", nil); status != http.StatusNoContent {
		t.Errorf("cancel pending request = %d, want 204", status)
	}
	if _, reqs := alice.list("/api/follow-requests"); len(reqs) != 0 {
		t.Errorf("requests after cancel = %v, want none", reqs)
	}
}

func TestFollowEdgeCases(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")

	if status, body := alice.do("POST", fmt.Sprintf("/api/users/%d/follow", aliceID), "", nil); status != http.StatusBadRequest {
		t.Errorf("follow yourself = %d %v, want 400", status, body)
	}
	for _, path := range []string{"/api/users/999/follow", "/api/users/abc/follow"} {
		if status, _ := alice.do("POST", path, "", nil); status != http.StatusNotFound {
			t.Errorf("POST %s = %d, want 404", path, status)
		}
		if status, _ := alice.do("DELETE", path, "", nil); status != http.StatusNotFound {
			t.Errorf("DELETE %s = %d, want 404", path, status)
		}
	}
	for _, path := range []string{"/api/users/999/followers", "/api/users/999/following"} {
		if status, _ := alice.list(path); status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, status)
		}
	}
	if status, _ := alice.do("POST", "/api/follow-requests/abc/accept", "", nil); status != http.StatusNotFound {
		t.Errorf("accept bad id = %d, want 404", status)
	}
	anon := newClient(t, srv.URL)
	for _, route := range [][2]string{
		{"POST", "/api/users/1/follow"}, {"DELETE", "/api/users/1/follow"}, {"GET", "/api/users/1/followers"},
		{"GET", "/api/users/1/following"}, {"GET", "/api/follow-requests"}, {"POST", "/api/follow-requests/1/accept"},
		{"POST", "/api/follow-requests/1/decline"},
	} {
		if status, _ := anon.do(route[0], route[1], "", nil); status != http.StatusUnauthorized {
			t.Errorf("%s %s without a session = %d, want 401", route[0], route[1], status)
		}
	}
}

func TestGoingPublicAcceptsPendingRequests(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	alice, aliceID := register(t, srv.URL, "alice@example.com", "Alice")
	bob, _ := register(t, srv.URL, "bob@example.com", "Bob")
	carol, _ := register(t, srv.URL, "carol@example.com", "Carol")
	alice.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	for _, c := range []*testClient{bob, carol} {
		c.do("POST", fmt.Sprintf("/api/users/%d/follow", aliceID), "", nil)
	}
	if _, reqs := alice.list("/api/follow-requests"); len(reqs) != 2 {
		t.Fatalf("pending requests = %v, want 2", reqs)
	}

	// Going private again does nothing to requests; going public accepts them.
	alice.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	if _, reqs := alice.list("/api/follow-requests"); len(reqs) != 2 {
		t.Fatalf("pending requests after re-private = %d, want 2", len(reqs))
	}
	if status, _ := alice.json("PATCH", "/api/me/privacy", `{"is_public": true}`); status != http.StatusOK {
		t.Fatalf("go public = %d", status)
	}
	if _, reqs := alice.list("/api/follow-requests"); len(reqs) != 0 {
		t.Errorf("pending requests after going public = %v, want none", reqs)
	}
	for name, c := range map[string]*testClient{"bob": bob, "carol": carol} {
		if s := followStatusOf(t, c, aliceID); s != "following" {
			t.Errorf("%s after alice went public: %q, want following", name, s)
		}
	}
	var pending int
	app.DB.QueryRow(`SELECT COUNT(*) FROM follows WHERE status = 'pending'`).Scan(&pending)
	if pending != 0 {
		t.Errorf("%d pending rows left", pending)
	}
}
