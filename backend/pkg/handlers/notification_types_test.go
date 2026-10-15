package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Every notification type reaches its recipient live over the WebSocket,
// with the actor and, for group types, the group (and event) it is about.
// The same notifications are then in GET /api/notifications, except the
// requests that were answered (those are removed).
func TestEveryNotificationTypeIsPushedLive(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	tom, tomID := register(t, srv.URL, "tom@example.com", "Tom") // the recipient, online
	ann, _ := register(t, srv.URL, "ann@example.com", "Ann")
	bob, _ := register(t, srv.URL, "bob@example.com", "Bob")
	cat, catID := register(t, srv.URL, "cat@example.com", "Cat")
	dan, _ := register(t, srv.URL, "dan@example.com", "Dan")
	eve, _ := register(t, srv.URL, "eve@example.com", "Eve")
	conn := connect(t, app, srv, tom, tomID)

	// expect reads Tom's next notification event and checks it.
	expect := func(typ, actor, groupTitle, eventTitle string) {
		t.Helper()
		ev, ok := next(conn, 3*time.Second, "notification")
		if !ok {
			t.Fatalf("%s: no live notification", typ)
		}
		p := ev.Payload
		a, _ := p["actor"].(map[string]any)
		g, _ := p["group"].(map[string]any)
		e, _ := p["event"].(map[string]any)
		switch {
		case p["type"] != typ:
			t.Errorf("got %v, want %s", p["type"], typ)
		case a["first_name"] != actor:
			t.Errorf("%s: actor %v, want %s", typ, a["first_name"], actor)
		case groupTitle != "" && g["title"] != groupTitle:
			t.Errorf("%s: group %v, want %s", typ, g, groupTitle)
		case eventTitle != "" && e["title"] != eventTitle:
			t.Errorf("%s: event %v, want %s", typ, e, eventTitle)
		case p["is_read"] != false || p["id"] == nil || p["created_at"] == nil:
			t.Errorf("%s: payload %v", typ, p)
		}
	}

	// new_follower: Tom is public, Ann follows.
	ann.do("POST", fmt.Sprintf("/api/users/%d/follow", tomID), "", nil)
	expect("new_follower", "Ann", "", "")

	// follow_request: Tom goes private, Bob asks.
	tom.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	bob.do("POST", fmt.Sprintf("/api/users/%d/follow", tomID), "", nil)
	expect("follow_request", "Bob", "", "")

	// follow_accepted: Tom asks private Cat, Cat accepts.
	cat.json("PATCH", "/api/me/privacy", `{"is_public": false}`)
	tom.do("POST", fmt.Sprintf("/api/users/%d/follow", catID), "", nil)
	cat.do("POST", fmt.Sprintf("/api/follow-requests/%d/accept", tomID), "", nil)
	expect("follow_accepted", "Cat", "", "")

	// group_invite: Dan invites Tom to Hikers.
	hikers := createGroup(t, dan, "Hikers")
	invite(t, dan, hikers, tomID)
	expect("group_invite", "Dan", "Hikers", "")

	// group_join_request: Tom's own group, Eve asks to join.
	mine := createGroup(t, tom, "Tom's club")
	if status, _ := eve.do("POST", fmt.Sprintf("/api/groups/%d/request", mine), "", nil); status != http.StatusCreated {
		t.Fatalf("eve asks to join = %d", status)
	}
	expect("group_join_request", "Eve", "Tom's club", "")

	// group_request_accepted: Tom asks to join Dan's Readers, Dan accepts.
	readers := createGroup(t, dan, "Readers")
	tom.do("POST", fmt.Sprintf("/api/groups/%d/request", readers), "", nil)
	_, reqs := dan.list(fmt.Sprintf("/api/groups/%d/requests", readers))
	dan.do("POST", fmt.Sprintf("/api/group-requests/%d/accept", int64(reqs[0]["id"].(float64))), "", nil)
	expect("group_request_accepted", "Dan", "Readers", "")

	// group_event: Dan creates an event in Readers, where Tom is a member now.
	when := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	if status, body := dan.json("POST", fmt.Sprintf("/api/groups/%d/events", readers),
		fmt.Sprintf(`{"title":"Book swap","description":"bring two","event_time":%q}`, when)); status != http.StatusCreated {
		t.Fatalf("create event = %d %v", status, body)
	}
	expect("group_event", "Dan", "Readers", "Book swap")

	// Everything that is not an answered request stays in the list.
	_, list := tom.list("/api/notifications")
	got := map[string]int{}
	for _, n := range list {
		got[n["type"].(string)]++
	}
	want := map[string]int{"new_follower": 1, "follow_request": 1, "follow_accepted": 1, "group_invite": 1,
		"group_join_request": 1, "group_request_accepted": 1, "group_event": 1}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("tom's notifications = %v, want %v", got, want)
	}
	if _, c := tom.get("/api/notifications/unread-count"); c["unread"] != float64(7) {
		t.Errorf("unread = %v, want 7", c["unread"])
	}
}
