package handlers

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// createPost sends POST /api/posts. viewers become repeated viewer_ids
// fields; image, when not nil, is sent as the "image" file.
func (c *testClient) createPost(content, privacy string, viewers []int64, image []byte) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("content", content)
	mw.WriteField("privacy", privacy)
	for _, id := range viewers {
		mw.WriteField("viewer_ids", fmt.Sprint(id))
	}
	if image != nil {
		fw, _ := mw.CreateFormFile("image", "photo.png")
		fw.Write(image)
	}
	mw.Close()
	return c.do("POST", "/api/posts", mw.FormDataContentType(), &buf)
}

func (c *testClient) status(path string) int {
	c.t.Helper()
	res, err := c.http.Get(c.base + path)
	if err != nil {
		c.t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

// follows makes follower an accepted follower of target (target is public
// at that moment, so it is instant).
func follows(t *testing.T, follower *testClient, targetID int64) {
	t.Helper()
	if status, body := follower.do("POST", fmt.Sprintf("/api/users/%d/follow", targetID), "", nil); status != http.StatusOK {
		t.Fatalf("follow %d = %d %v", targetID, status, body)
	}
}

func TestPostVisibilityMatrix(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	author, authorID := register(t, srv.URL, "author@example.com", "Author")
	chosen, chosenID := register(t, srv.URL, "chosen@example.com", "Chosen") // follower, picked for the private post
	other, _ := register(t, srv.URL, "other@example.com", "Other")           // follower, not picked
	stranger, _ := register(t, srv.URL, "stranger@example.com", "Stranger")  // does not follow
	follows(t, chosen, authorID)
	follows(t, other, authorID)

	type created struct{ id, image string }
	newPost := func(privacy string, viewers []int64) created {
		t.Helper()
		status, p := author.createPost("a "+privacy+" post 👋", privacy, viewers, tinyPNG(t))
		if status != http.StatusCreated {
			t.Fatalf("create %s post = %d %v", privacy, status, p)
		}
		if p["privacy"] != privacy || p["comment_count"] != float64(0) || !strings.HasPrefix(p["image_path"].(string), "/uploads/") {
			t.Errorf("created post = %v", p)
		}
		if a, _ := p["author"].(map[string]any); a["first_name"] != "Author" || a["email"] != nil {
			t.Errorf("author field = %v", p["author"])
		}
		return created{fmt.Sprint(int64(p["id"].(float64))), p["image_path"].(string)}
	}
	posts := map[string]created{
		"public":         newPost("public", nil),
		"almost_private": newPost("almost_private", nil),
		"private":        newPost("private", []int64{chosenID}),
	}

	want := map[string]map[string]bool{ // viewer -> privacy -> may see
		"author":   {"public": true, "almost_private": true, "private": true},
		"chosen":   {"public": true, "almost_private": true, "private": true},
		"other":    {"public": true, "almost_private": true, "private": false},
		"stranger": {"public": true, "almost_private": false, "private": false},
	}
	clients := map[string]*testClient{"author": author, "chosen": chosen, "other": other, "stranger": stranger}
	for who, c := range clients {
		for privacy, p := range posts {
			wantStatus := http.StatusNotFound
			if want[who][privacy] {
				wantStatus = http.StatusOK
			}
			if got := c.status("/api/posts/" + p.id); got != wantStatus {
				t.Errorf("%s GET %s post = %d, want %d", who, privacy, got, wantStatus)
			}
			if got := c.status(p.image); got != wantStatus {
				t.Errorf("%s GET %s post image = %d, want %d", who, privacy, got, wantStatus)
			}
		}
		// The profile list holds exactly the visible posts.
		_, list := c.list(fmt.Sprintf("/api/users/%d/posts", authorID))
		visible := 0
		for _, ok := range want[who] {
			if ok {
				visible++
			}
		}
		if len(list) != visible {
			t.Errorf("%s sees %d posts on the profile, want %d", who, len(list), visible)
		}
	}

	// Unfollowing removes access to almost_private and private posts at once.
	chosen.do("DELETE", fmt.Sprintf("/api/users/%d/follow", authorID), "", nil)
	for _, privacy := range []string{"almost_private", "private"} {
		if got := chosen.status("/api/posts/" + posts[privacy].id); got != http.StatusNotFound {
			t.Errorf("ex-follower GET %s post = %d, want 404", privacy, got)
		}
		if got := chosen.status(posts[privacy].image); got != http.StatusNotFound {
			t.Errorf("ex-follower GET %s image = %d, want 404", privacy, got)
		}
	}

	// Avatars stay visible to every logged-in user.
	_, me := author.form("PATCH", "/api/me", map[string]string{}, tinyPNG(t))
	if got := stranger.status(me["avatar_path"].(string)); got != http.StatusOK {
		t.Errorf("stranger GET author's avatar = %d, want 200", got)
	}
	if got := newClient(t, srv.URL).status(posts["public"].image); got != http.StatusUnauthorized {
		t.Errorf("anonymous GET public post image = %d, want 401", got)
	}
}

func TestPrivateProfileHidesPostList(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	author, authorID := register(t, srv.URL, "author@example.com", "Author")
	fan, _ := register(t, srv.URL, "fan@example.com", "Fan")
	stranger, _ := register(t, srv.URL, "stranger@example.com", "Stranger")
	follows(t, fan, authorID)
	author.createPost("hello", "public", nil, tinyPNG(t))
	author.json("PATCH", "/api/me/privacy", `{"is_public": false}`)

	path := fmt.Sprintf("/api/users/%d/posts", authorID)
	if status, _ := stranger.list(path); status != http.StatusForbidden {
		t.Errorf("stranger lists a private profile's posts = %d, want 403", status)
	}
	if status, list := fan.list(path); status != http.StatusOK || len(list) != 1 {
		t.Errorf("follower lists posts = %d %v", status, list)
	}
	if status, list := author.list(path); status != http.StatusOK || len(list) != 1 {
		t.Errorf("owner lists posts = %d %v", status, list)
	}
	// The profile's privacy wins over the post's: a public post of a private
	// profile is hidden from non-followers everywhere, image included.
	_, list := author.list(path)
	postID := fmt.Sprint(int64(list[0]["id"].(float64)))
	image := list[0]["image_path"].(string)
	if got := stranger.status("/api/posts/" + postID); got != http.StatusNotFound {
		t.Errorf("stranger GET a private profile's public post = %d, want 404", got)
	}
	if got := stranger.status(image); got != http.StatusNotFound {
		t.Errorf("stranger GET a private profile's public post image = %d, want 404", got)
	}
	if got := fan.status("/api/posts/" + postID); got != http.StatusOK {
		t.Errorf("follower GET a private profile's public post = %d, want 200", got)
	}
	// Going public again shows it to everyone.
	author.json("PATCH", "/api/me/privacy", `{"is_public": true}`)
	if got := stranger.status("/api/posts/" + postID); got != http.StatusOK {
		t.Errorf("stranger GET the public post after the profile went public = %d, want 200", got)
	}
	if status, _ := stranger.list("/api/users/999/posts"); status != http.StatusNotFound {
		t.Errorf("unknown user's posts = %d, want 404", status)
	}
}

func TestCreatePostValidation(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	author, authorID := register(t, srv.URL, "author@example.com", "Author")
	fan, fanID := register(t, srv.URL, "fan@example.com", "Fan")
	_, strangerID := register(t, srv.URL, "stranger@example.com", "Stranger")
	pending, pendingID := register(t, srv.URL, "pending@example.com", "Pending")
	follows(t, fan, authorID)
	author.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	pending.do("POST", fmt.Sprintf("/api/users/%d/follow", authorID), "", nil) // stays pending

	bad := []struct {
		name    string
		content string
		privacy string
		viewers []int64
		image   []byte
	}{
		{"empty post", "  ", "public", nil, nil},
		{"unknown privacy", "hi", "friends", nil, nil},
		{"missing privacy", "hi", "", nil, nil},
		{"private without viewers", "hi", "private", nil, nil},
		{"viewers on a public post", "hi", "public", []int64{fanID}, nil},
		{"stranger as viewer", "hi", "private", []int64{fanID, strangerID}, tinyPNG(t)},
		{"pending follower as viewer", "hi", "private", []int64{pendingID}, nil},
		{"unknown user as viewer", "hi", "private", []int64{999}, nil},
		{"self as viewer", "hi", "private", []int64{authorID}, nil},
		{"too long", strings.Repeat("x", 5001), "public", nil, nil},
		{"not an image", "hi", "public", nil, []byte("just text")},
	}
	for _, c := range bad {
		if status, body := author.createPost(c.content, c.privacy, c.viewers, c.image); status != http.StatusBadRequest || body["error"] == nil {
			t.Errorf("%s: %d %v, want 400 with a message", c.name, status, body)
		}
	}
	// Rejected posts leave neither rows nor files behind.
	var n int
	app.DB.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&n)
	files, _ := os.ReadDir(app.UploadDir)
	if n != 0 || len(files) != 0 {
		t.Errorf("after rejected posts: %d rows, %d files", n, len(files))
	}

	// Text only, image only, and a private post with a duplicated viewer.
	if status, p := author.createPost("text only", "almost_private", nil, nil); status != http.StatusCreated || p["image_path"] != "" {
		t.Errorf("text-only post = %d %v", status, p)
	}
	if status, p := author.createPost("", "public", nil, tinyPNG(t)); status != http.StatusCreated || p["content"] != "" {
		t.Errorf("image-only post = %d %v", status, p)
	}
	status, p := author.createPost("for fan", "private", []int64{fanID, fanID}, nil)
	if status != http.StatusCreated {
		t.Fatalf("private post = %d %v", status, p)
	}
	app.DB.QueryRow(`SELECT COUNT(*) FROM post_viewers WHERE post_id = ?`, p["id"]).Scan(&n)
	if n != 1 {
		t.Errorf("post_viewers rows = %d, want 1 (duplicates dropped)", n)
	}

	for _, path := range []string{"/api/posts/abc", "/api/posts/999"} {
		if got := author.status(path); got != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, got)
		}
	}
	anon := newClient(t, srv.URL)
	if status, _ := anon.createPost("hi", "public", nil, nil); status != http.StatusUnauthorized {
		t.Errorf("anonymous create = %d, want 401", status)
	}
	if got := anon.status("/api/posts/1"); got != http.StatusUnauthorized {
		t.Errorf("anonymous GET post = %d, want 401", got)
	}
}

func TestUserPostsPaging(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	author, authorID := register(t, srv.URL, "author@example.com", "Author")
	for i := 0; i < 25; i++ {
		author.createPost(fmt.Sprintf("post %d", i), "public", nil, nil)
	}
	path := fmt.Sprintf("/api/users/%d/posts", authorID)
	_, page1 := author.list(path)
	if len(page1) != 20 || page1[0]["content"] != "post 24" {
		t.Fatalf("page 1: %d posts, first %v", len(page1), page1[0]["content"])
	}
	last := int64(page1[19]["id"].(float64))
	_, page2 := author.list(fmt.Sprintf("%s?before=%d", path, last))
	if len(page2) != 5 || page2[4]["content"] != "post 0" {
		t.Errorf("page 2: %d posts", len(page2))
	}
	if status, _ := author.list(path + "?before=x"); status != http.StatusBadRequest {
		t.Errorf("before=x = %d, want 400", status)
	}
}
