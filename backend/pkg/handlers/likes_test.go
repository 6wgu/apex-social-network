package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLikesFollowPostVisibility(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	author, authorID := register(t, srv.URL, "author@example.com", "Author")
	chosen, chosenID := register(t, srv.URL, "chosen@example.com", "Chosen")
	stranger, _ := register(t, srv.URL, "stranger@example.com", "Stranger")
	follows(t, chosen, authorID)

	_, pub := author.createPost("public", "public", nil, nil)
	_, prv := author.createPost("private", "private", []int64{chosenID}, nil)
	pubID, prvID := int64(pub["id"].(float64)), int64(prv["id"].(float64))
	if pub["like_count"] != float64(0) || pub["liked_by_me"] != false {
		t.Fatalf("new post likes = %v %v", pub["like_count"], pub["liked_by_me"])
	}

	like := func(c *testClient, postID int64, method string) (int, map[string]any) {
		return c.do(method, fmt.Sprintf("/api/posts/%d/like", postID), "", nil)
	}

	// Liking is counted once per user, and liking twice is not an error.
	if status, s := like(stranger, pubID, "POST"); status != http.StatusOK || s["like_count"] != float64(1) || s["liked_by_me"] != true {
		t.Fatalf("stranger likes the public post = %d %v", status, s)
	}
	if status, s := like(stranger, pubID, "POST"); status != http.StatusOK || s["like_count"] != float64(1) {
		t.Errorf("liking twice = %d %v, want the count to stay 1", status, s)
	}
	if status, s := like(author, pubID, "POST"); status != http.StatusOK || s["like_count"] != float64(2) {
		t.Errorf("author likes their own post = %d %v", status, s)
	}

	// Each viewer sees the shared count and their own liked_by_me.
	if _, p := stranger.get(fmt.Sprintf("/api/posts/%d", pubID)); p["like_count"] != float64(2) || p["liked_by_me"] != true {
		t.Errorf("stranger sees %v %v", p["like_count"], p["liked_by_me"])
	}
	if _, p := chosen.get(fmt.Sprintf("/api/posts/%d", pubID)); p["like_count"] != float64(2) || p["liked_by_me"] != false {
		t.Errorf("chosen sees %v %v", p["like_count"], p["liked_by_me"])
	}

	// Unliking removes only your own like; unliking again is not an error.
	if status, s := like(stranger, pubID, "DELETE"); status != http.StatusOK || s["like_count"] != float64(1) || s["liked_by_me"] != false {
		t.Errorf("stranger unlikes = %d %v", status, s)
	}
	if status, _ := like(stranger, pubID, "DELETE"); status != http.StatusOK {
		t.Errorf("unliking twice = %d, want 200", status)
	}

	// A post you may not see cannot be liked (404, like a missing post).
	if status, _ := like(stranger, prvID, "POST"); status != http.StatusNotFound {
		t.Errorf("stranger likes a hidden post = %d, want 404", status)
	}
	if status, s := like(chosen, prvID, "POST"); status != http.StatusOK || s["like_count"] != float64(1) {
		t.Errorf("chosen likes the private post = %d %v", status, s)
	}
	if status, _ := like(chosen, 999999, "POST"); status != http.StatusNotFound {
		t.Errorf("liking a missing post = %d, want 404", status)
	}

	// The feed carries the counts too.
	_, feed := author.list("/api/feed")
	for _, p := range feed {
		if p["id"] == float64(prvID) && (p["like_count"] != float64(1) || p["liked_by_me"] != false) {
			t.Errorf("feed private post likes = %v %v", p["like_count"], p["liked_by_me"])
		}
	}
}
