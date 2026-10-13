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

// comment sends POST /api/posts/{id}/comments with an optional image.
func (c *testClient) comment(postID any, content string, image []byte) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("content", content)
	if image != nil {
		fw, _ := mw.CreateFormFile("image", "c.gif")
		fw.Write(image)
	}
	mw.Close()
	return c.do("POST", fmt.Sprintf("/api/posts/%v/comments", postID), mw.FormDataContentType(), &buf)
}

func TestCommentsFollowPostVisibility(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	author, authorID := register(t, srv.URL, "author@example.com", "Author")
	chosen, chosenID := register(t, srv.URL, "chosen@example.com", "Chosen")
	other, _ := register(t, srv.URL, "other@example.com", "Other")
	stranger, _ := register(t, srv.URL, "stranger@example.com", "Stranger")
	follows(t, chosen, authorID)
	follows(t, other, authorID)

	_, pub := author.createPost("public", "public", nil, nil)
	_, prv := author.createPost("private", "private", []int64{chosenID}, nil)
	pubID, prvID := int64(pub["id"].(float64)), int64(prv["id"].(float64))

	// Anyone who can see the public post can comment, with text, an image, or both.
	if status, c := stranger.comment(pubID, "nice 👍", nil); status != http.StatusCreated || c["content"] != "nice 👍" || c["post_id"] != float64(pubID) {
		t.Fatalf("stranger comments on a public post = %d %v", status, c)
	}
	status, withImage := other.comment(pubID, "", tinyPNG(t))
	if status != http.StatusCreated || !strings.HasPrefix(withImage["image_path"].(string), "/uploads/") {
		t.Fatalf("image-only comment = %d %v", status, withImage)
	}
	if a, _ := withImage["author"].(map[string]any); a["first_name"] != "Other" || a["email"] != nil {
		t.Errorf("comment author = %v", withImage["author"])
	}

	// The private post: only the author and the chosen follower.
	status, privComment := chosen.comment(prvID, "secret", tinyPNG(t))
	if status != http.StatusCreated {
		t.Fatalf("chosen comments on the private post = %d %v", status, privComment)
	}
	privImage := privComment["image_path"].(string)
	for name, c := range map[string]*testClient{"other": other, "stranger": stranger} {
		if status, _ := c.comment(prvID, "let me in", nil); status != http.StatusNotFound {
			t.Errorf("%s comments on a hidden post = %d, want 404", name, status)
		}
		if status, _ := c.list(fmt.Sprintf("/api/posts/%d/comments", prvID)); status != http.StatusNotFound {
			t.Errorf("%s lists a hidden post's comments = %d, want 404", name, status)
		}
		if got := c.status(privImage); got != http.StatusNotFound {
			t.Errorf("%s GET a hidden post's comment image = %d, want 404", name, got)
		}
	}
	for name, c := range map[string]*testClient{"author": author, "chosen": chosen} {
		if status, list := c.list(fmt.Sprintf("/api/posts/%d/comments", prvID)); status != http.StatusOK || len(list) != 1 {
			t.Errorf("%s lists the private post's comments = %d %v", name, status, list)
		}
		if got := c.status(privImage); got != http.StatusOK {
			t.Errorf("%s GET the comment image = %d, want 200", name, got)
		}
	}

	// Oldest first, with a comment count on the post.
	_, list := author.list(fmt.Sprintf("/api/posts/%d/comments", pubID))
	if len(list) != 2 || list[0]["content"] != "nice 👍" {
		t.Errorf("public post comments = %v", list)
	}
	if _, p := author.get(fmt.Sprintf("/api/posts/%d", pubID)); p["comment_count"] != float64(2) {
		t.Errorf("comment_count = %v, want 2", p["comment_count"])
	}

	// Losing access to the post means losing access to its comments.
	chosen.do("DELETE", fmt.Sprintf("/api/users/%d/follow", authorID), "", nil)
	if status, _ := chosen.list(fmt.Sprintf("/api/posts/%d/comments", prvID)); status != http.StatusNotFound {
		t.Errorf("ex-follower lists private comments = %d, want 404", status)
	}
	if got := chosen.status(privImage); got != http.StatusNotFound {
		t.Errorf("ex-follower GET their own comment image on a hidden post = %d, want 404", got)
	}
}

func TestCommentValidation(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	author, _ := register(t, srv.URL, "author@example.com", "Author")
	_, p := author.createPost("hi", "public", nil, nil)
	postID := int64(p["id"].(float64))

	for name, c := range map[string]struct {
		content string
		image   []byte
	}{
		"empty":     {"   ", nil},
		"too long":  {strings.Repeat("x", 2001), nil},
		"not image": {"hi", []byte("plain text")},
	} {
		if status, body := author.comment(postID, c.content, c.image); status != http.StatusBadRequest || body["error"] == nil {
			t.Errorf("%s: %d %v, want 400", name, status, body)
		}
	}
	files, _ := os.ReadDir(app.UploadDir)
	if len(files) != 0 {
		t.Errorf("rejected comments left %d files", len(files))
	}
	for _, id := range []any{999, "abc"} {
		if status, _ := author.comment(id, "hi", nil); status != http.StatusNotFound {
			t.Errorf("comment on post %v = %d, want 404", id, status)
		}
	}
	if status, _ := author.list(fmt.Sprintf("/api/posts/%d/comments?after=x", postID)); status != http.StatusBadRequest {
		t.Errorf("after=x = %d, want 400", status)
	}

	// Paging: 55 comments come back as 50 + 5.
	for i := 0; i < 55; i++ {
		author.comment(postID, fmt.Sprintf("c%d", i), nil)
	}
	_, page1 := author.list(fmt.Sprintf("/api/posts/%d/comments", postID))
	last := int64(page1[len(page1)-1]["id"].(float64))
	_, page2 := author.list(fmt.Sprintf("/api/posts/%d/comments?after=%d", postID, last))
	if len(page1) != 50 || len(page2) != 5 || page2[4]["content"] != "c54" {
		t.Errorf("paging: %d + %d", len(page1), len(page2))
	}

	anon := newClient(t, srv.URL)
	if status, _ := anon.comment(postID, "hi", nil); status != http.StatusUnauthorized {
		t.Errorf("anonymous comment = %d, want 401", status)
	}
	if status, _ := anon.list(fmt.Sprintf("/api/posts/%d/comments", postID)); status != http.StatusUnauthorized {
		t.Errorf("anonymous list = %d, want 401", status)
	}
}

func TestFeed(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	me, meID := register(t, srv.URL, "me@example.com", "Me")
	pub, pubID := register(t, srv.URL, "pub@example.com", "Pub")     // public, I follow
	priv, privID := register(t, srv.URL, "priv@example.com", "Priv") // private, I follow
	shy, _ := register(t, srv.URL, "shy@example.com", "Shy")         // private, I don't follow
	open, _ := register(t, srv.URL, "open@example.com", "Open")      // public, I don't follow
	follows(t, me, pubID)
	follows(t, me, privID) // both public at this moment: instant
	follows(t, priv, meID) // so I can pick Priv for my own private post
	priv.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	shy.json("PATCH", "/api/me/privacy", `{"is_public": false}`)

	post := func(c *testClient, text, privacy string, viewers []int64) {
		t.Helper()
		if status, body := c.createPost(text, privacy, viewers, nil); status != http.StatusCreated {
			t.Fatalf("post %q = %d %v", text, status, body)
		}
	}
	post(me, "mine private", "private", []int64{privID})
	post(pub, "pub public", "public", nil)
	post(pub, "pub almost", "almost_private", nil)
	post(pub, "pub private (not for me)", "private", []int64{newFan(t, srv.URL, pubID)})
	post(priv, "priv public", "public", nil)
	post(priv, "priv almost", "almost_private", nil)
	post(shy, "shy public", "public", nil)
	post(shy, "shy almost", "almost_private", nil)
	post(open, "open public", "public", nil)
	post(open, "open almost", "almost_private", nil)

	_, feed := me.list("/api/feed")
	var got []string
	for _, p := range feed {
		got = append(got, p["content"].(string))
	}
	want := []string{"open public", "priv almost", "priv public", "pub almost", "pub public", "mine private"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("feed =\n %q\nwant\n %q", got, want)
	}
	if feed[0]["comment_count"] != float64(0) {
		t.Errorf("feed post comment_count = %v", feed[0]["comment_count"])
	}

	// Paging, newest first.
	for i := 0; i < 25; i++ {
		post(me, fmt.Sprintf("p%d", i), "public", nil)
	}
	_, page1 := me.list("/api/feed")
	last := int64(page1[len(page1)-1]["id"].(float64))
	_, page2 := me.list(fmt.Sprintf("/api/feed?before=%d", last))
	if len(page1) != 20 || page1[0]["content"] != "p24" || len(page2) != 11 {
		t.Errorf("feed paging: %d (first %v) + %d", len(page1), page1[0]["content"], len(page2))
	}
	if status, _ := me.list("/api/feed?before=x"); status != http.StatusBadRequest {
		t.Errorf("before=x = %d, want 400", status)
	}
	if status, _ := newClient(t, srv.URL).list("/api/feed"); status != http.StatusUnauthorized {
		t.Errorf("anonymous feed = %d, want 401", status)
	}
}

// newFan registers a user who follows targetID and returns the fan's id.
func newFan(t *testing.T, base string, targetID int64) int64 {
	t.Helper()
	fan, fanID := register(t, base, "fan@example.com", "Fan")
	follows(t, fan, targetID)
	return fanID
}
