package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// New posts, comments and like counts are pushed to online users who may
// see the post, and to nobody else.
func TestPostActivityIsPushedToViewersOnly(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	ben, benID := register(t, srv.URL, "ben@example.com", "Ben")
	cara, caraID := register(t, srv.URL, "cara@example.com", "Cara")
	ben.do("POST", fmt.Sprintf("/api/users/%d/follow", annID), "", nil)
	annConn := connect(t, app, srv, ann, annID)
	benConn := connect(t, app, srv, ben, benID)
	caraConn := connect(t, app, srv, cara, caraID)

	// Followers only: Ben (a follower) may see it, Cara may not.
	status, p := ann.createPost("for my followers", privacyAlmostPrivate, nil, nil)
	if status != http.StatusCreated {
		t.Fatalf("create post = %d %v", status, p)
	}
	postID := p["id"]
	if ev, ok := next(benConn, 2*time.Second, eventPostCreated); !ok || ev.Payload["post_id"] != postID {
		t.Fatalf("ben post_created = %+v (ok %v)", ev, ok)
	}

	if status, c := ben.comment(postID, "nice", nil); status != http.StatusCreated {
		t.Fatalf("comment = %d %v", status, c)
	}
	if ev, ok := next(annConn, 2*time.Second, eventCommentCreated); !ok || ev.Payload["content"] != "nice" || ev.Payload["post_id"] != postID {
		t.Fatalf("ann comment_created = %+v (ok %v)", ev, ok)
	}

	if status, s := ben.do("POST", fmt.Sprintf("/api/posts/%v/like", postID), "", nil); status != http.StatusOK {
		t.Fatalf("like = %d %v", status, s)
	}
	if ev, ok := next(annConn, 2*time.Second, eventPostLikes); !ok || ev.Payload["like_count"] != float64(1) {
		t.Fatalf("ann post_likes = %+v (ok %v)", ev, ok)
	}

	// Cara got none of it (a timed-out read is the last on this connection).
	if ev, ok := next(caraConn, 500*time.Millisecond, eventPostCreated, eventCommentCreated, eventPostLikes); ok {
		t.Errorf("cara received %+v about a post she cannot see", ev)
	}
}
