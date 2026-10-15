package handlers

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// createGroup sends POST /api/groups and returns the new group's id.
func createGroup(t *testing.T, c *testClient, title string) int64 {
	t.Helper()
	status, g := c.json("POST", "/api/groups", fmt.Sprintf(`{"title": %q, "description": "About %s"}`, title, title))
	if status != http.StatusCreated {
		t.Fatalf("create group %q = %d %v", title, status, g)
	}
	return int64(g["id"].(float64))
}

// groupPost sends POST /api/groups/{id}/posts with an optional image.
func (c *testClient) groupPost(groupID int64, content string, image []byte) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("content", content)
	if image != nil {
		fw, _ := mw.CreateFormFile("image", "g.png")
		fw.Write(image)
	}
	mw.Close()
	return c.do("POST", fmt.Sprintf("/api/groups/%d/posts", groupID), mw.FormDataContentType(), &buf)
}

// invite has a member invite userID and returns the invitation id.
func invite(t *testing.T, member *testClient, groupID, userID int64) int64 {
	t.Helper()
	status, body := member.json("POST", fmt.Sprintf("/api/groups/%d/invite", groupID), fmt.Sprintf(`{"user_id": %d}`, userID))
	if status != http.StatusCreated {
		t.Fatalf("invite %d to group %d = %d %v", userID, groupID, status, body)
	}
	return int64(body["id"].(float64))
}

// joinByInvite makes c a member of groupID through an invitation from member.
func joinByInvite(t *testing.T, member, c *testClient, groupID, userID int64) {
	t.Helper()
	inviteID := invite(t, member, groupID, userID)
	if status, body := c.do("POST", fmt.Sprintf("/api/group-invites/%d/accept", inviteID), "", nil); status != http.StatusNoContent {
		t.Fatalf("accept invitation = %d %v", status, body)
	}
}

func myStatus(t *testing.T, c *testClient, groupID int64) string {
	t.Helper()
	status, g := c.get(fmt.Sprintf("/api/groups/%d", groupID))
	if status != http.StatusOK {
		t.Fatalf("GET group %d = %d %v", groupID, status, g)
	}
	return g["my_status"].(string)
}

// notificationsOf lists a user's notifications of one type.
func notificationsOf(t *testing.T, c *testClient, typ string) []map[string]any {
	t.Helper()
	status, list := c.list("/api/notifications")
	if status != http.StatusOK {
		t.Fatalf("notifications = %d", status)
	}
	var out []map[string]any
	for _, n := range list {
		if n["type"] == typ {
			out = append(out, n)
		}
	}
	return out
}

// Gate 3: non-members can't read group posts, comments or events, even via
// the API, and can't write them either.
func TestGroupContentIsMembersOnly(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	creator, creatorID := register(t, srv.URL, "creator@example.com", "Creator")
	outsider, outsiderID := register(t, srv.URL, "outsider@example.com", "Outsider")
	follows(t, outsider, creatorID) // following the creator gives no access to the group
	gid := createGroup(t, creator, "Hiking club")
	base := fmt.Sprintf("/api/groups/%d", gid)

	status, post := creator.groupPost(gid, "Members only 🥾", tinyPNG(t))
	if status != http.StatusCreated {
		t.Fatalf("group post = %d %v", status, post)
	}
	if post["group_id"] != float64(gid) {
		t.Errorf("group post group_id = %v, want %d", post["group_id"], gid)
	}
	postPath := fmt.Sprintf("/api/posts/%v", post["id"])
	image := post["image_path"].(string)
	status, c := creator.comment(post["id"], "first!", tinyPNG(t))
	if status != http.StatusCreated {
		t.Fatalf("comment on group post = %d %v", status, c)
	}
	commentImage := c["image_path"].(string)
	status, ev := creator.json("POST", base+"/events", fmt.Sprintf(`{"title":"Walk","description":"Up the hill","event_time":%q}`,
		time.Now().Add(48*time.Hour).UTC().Format(time.RFC3339)))
	if status != http.StatusCreated {
		t.Fatalf("create event = %d %v", status, ev)
	}
	eventID := ev["id"]

	// The outsider can see that the group exists, but nothing inside it.
	status, g := outsider.get(base)
	if status != http.StatusOK || g["my_status"] != "none" || g["member_count"] != float64(1) {
		t.Errorf("outsider GET group = %d %v", status, g)
	}
	if _, ok := g["members"]; ok {
		t.Errorf("outsider sees the member list: %v", g["members"])
	}
	checks := []struct {
		method, path, body string
		want               int
	}{
		{"GET", base + "/posts", "", http.StatusForbidden},
		{"GET", base + "/events", "", http.StatusForbidden},
		{"GET", base + "/requests", "", http.StatusForbidden},
		{"GET", postPath, "", http.StatusNotFound},
		{"GET", postPath + "/comments", "", http.StatusNotFound},
		{"GET", image, "", http.StatusNotFound},
		{"GET", commentImage, "", http.StatusNotFound},
		{"POST", base + "/events", `{"title":"x","description":"y","event_time":"2099-01-01T10:00:00Z"}`, http.StatusForbidden},
		{"POST", base + "/invite", fmt.Sprintf(`{"user_id": %d}`, outsiderID), http.StatusForbidden},
		{"POST", fmt.Sprintf("/api/events/%v/respond", eventID), `{"response":"going"}`, http.StatusNotFound},
	}
	for _, ch := range checks {
		var got int
		if ch.body != "" {
			got, _ = outsider.json(ch.method, ch.path, ch.body)
		} else {
			got = outsider.status(ch.path)
		}
		if got != ch.want {
			t.Errorf("outsider %s %s = %d, want %d", ch.method, ch.path, got, ch.want)
		}
	}
	if status, _ := outsider.groupPost(gid, "let me in", nil); status != http.StatusForbidden {
		t.Errorf("outsider posts in the group = %d, want 403", status)
	}
	if status, _ := outsider.comment(post["id"], "hi", nil); status != http.StatusNotFound {
		t.Errorf("outsider comments on a group post = %d, want 404", status)
	}

	// Group posts stay in the group: not in anyone's feed or on the profile.
	for who, c := range map[string]*testClient{"creator": creator, "outsider": outsider} {
		if _, feed := c.list("/api/feed"); len(feed) != 0 {
			t.Errorf("%s's feed holds %d posts, want 0", who, len(feed))
		}
		if _, list := c.list(fmt.Sprintf("/api/users/%d/posts", creatorID)); len(list) != 0 {
			t.Errorf("%s sees %d posts on the creator's profile, want 0", who, len(list))
		}
	}

	// Once a member, the same user sees everything.
	joinByInvite(t, creator, outsider, gid, outsiderID)
	for _, path := range []string{base + "/posts", base + "/events", postPath, postPath + "/comments", image, commentImage} {
		if got := outsider.status(path); got != http.StatusOK {
			t.Errorf("member GET %s = %d, want 200", path, got)
		}
	}
	if _, list := outsider.list(base + "/posts"); len(list) != 1 {
		t.Errorf("member sees %d group posts, want 1", len(list))
	}
	if status, _ := outsider.groupPost(gid, "Thanks for having me", nil); status != http.StatusCreated {
		t.Errorf("new member posts = %d, want 201", status)
	}
	if _, g := outsider.get(base); g["my_status"] != "member" || len(g["members"].([]any)) != 2 {
		t.Errorf("member GET group = %v", g)
	}
}

// Gate 3: any member can invite; the invited user accepts or refuses.
func TestGroupInvitations(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	creator, creatorID := register(t, srv.URL, "creator@example.com", "Creator")
	member, memberID := register(t, srv.URL, "member@example.com", "Member")
	guest, guestID := register(t, srv.URL, "guest@example.com", "Guest")
	other, otherID := register(t, srv.URL, "other@example.com", "Other")
	gid := createGroup(t, creator, "Book club")
	invitePath := fmt.Sprintf("/api/groups/%d/invite", gid)
	joinByInvite(t, creator, member, gid, memberID)

	// A plain member (not the creator) invites the guest.
	inviteID := invite(t, member, gid, guestID)
	if got := myStatus(t, guest, gid); got != "invited" {
		t.Errorf("guest status = %q, want invited", got)
	}
	notes := notificationsOf(t, guest, "group_invite")
	if len(notes) != 1 {
		t.Fatalf("guest has %d group_invite notifications, want 1", len(notes))
	}
	if g := notes[0]["group"].(map[string]any); g["title"] != "Book club" || g["id"] != float64(gid) {
		t.Errorf("notification group = %v", g)
	}
	if a := notes[0]["actor"].(map[string]any); a["id"] != float64(memberID) {
		t.Errorf("notification actor = %v, want the member who invited", a)
	}
	for who, c := range map[string]*testClient{"creator": creator, "member": member, "other": other} {
		if n := notificationsOf(t, c, "group_invite"); len(n) != 0 {
			t.Errorf("%s got a group_invite notification", who)
		}
	}
	if _, g := creator.get(fmt.Sprintf("/api/groups/%d", gid)); len(g["invited_users"].([]any)) != 1 {
		t.Errorf("invited_users = %v, want the guest", g["invited_users"])
	}

	// Duplicates and bad targets.
	for _, tc := range []struct {
		body string
		want int
	}{
		{fmt.Sprintf(`{"user_id": %d}`, guestID), http.StatusConflict},   // already invited
		{fmt.Sprintf(`{"user_id": %d}`, memberID), http.StatusConflict},  // already a member
		{fmt.Sprintf(`{"user_id": %d}`, creatorID), http.StatusConflict}, // the creator is a member
		{`{"user_id": 999}`, http.StatusNotFound},
		{`{}`, http.StatusBadRequest},
	} {
		if status, body := member.json("POST", invitePath, tc.body); status != tc.want {
			t.Errorf("invite %s = %d %v, want %d", tc.body, status, body, tc.want)
		}
	}

	// Only the invitee can answer.
	accept := fmt.Sprintf("/api/group-invites/%d/accept", inviteID)
	decline := fmt.Sprintf("/api/group-invites/%d/decline", inviteID)
	for _, c := range []*testClient{creator, member, other} {
		if status, _ := c.do("POST", accept, "", nil); status != http.StatusNotFound {
			t.Errorf("someone else accepts the guest's invitation = %d, want 404", status)
		}
	}

	// Declining removes the invitation and its notification; a new one can follow.
	if status, _ := guest.do("POST", decline, "", nil); status != http.StatusNoContent {
		t.Fatalf("decline = %d", status)
	}
	if got := myStatus(t, guest, gid); got != "none" {
		t.Errorf("after decline status = %q, want none", got)
	}
	if n := notificationsOf(t, guest, "group_invite"); len(n) != 0 {
		t.Errorf("declined invitation still in the bell: %v", n)
	}
	if status, _ := guest.do("POST", decline, "", nil); status != http.StatusNotFound {
		t.Errorf("answering twice = %d, want 404", status)
	}

	// Accepting makes the guest a member, who can then invite too.
	inviteID = invite(t, creator, gid, guestID)
	if status, _ := guest.do("POST", fmt.Sprintf("/api/group-invites/%d/accept", inviteID), "", nil); status != http.StatusNoContent {
		t.Fatalf("accept = %d", status)
	}
	if got := myStatus(t, guest, gid); got != "member" {
		t.Errorf("after accept status = %q, want member", got)
	}
	if n := notificationsOf(t, guest, "group_invite"); len(n) != 0 {
		t.Errorf("accepted invitation still in the bell: %v", n)
	}
	invite(t, guest, gid, otherID)
}

// Gate 3: only the creator can accept or refuse join requests.
func TestGroupJoinRequests(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	creator, _ := register(t, srv.URL, "creator@example.com", "Creator")
	member, memberID := register(t, srv.URL, "member@example.com", "Member")
	asker, askerID := register(t, srv.URL, "asker@example.com", "Asker")
	gid := createGroup(t, creator, "Chess")
	joinByInvite(t, creator, member, gid, memberID)
	requestPath := fmt.Sprintf("/api/groups/%d/request", gid)
	listPath := fmt.Sprintf("/api/groups/%d/requests", gid)

	status, req := asker.do("POST", requestPath, "", nil)
	if status != http.StatusCreated {
		t.Fatalf("request to join = %d %v", status, req)
	}
	requestID := int64(req["id"].(float64))
	if status, _ := asker.do("POST", requestPath, "", nil); status != http.StatusConflict {
		t.Errorf("second request = %d, want 409", status)
	}
	if status, _ := member.do("POST", requestPath, "", nil); status != http.StatusConflict {
		t.Errorf("member asks to join = %d, want 409", status)
	}
	if got := myStatus(t, asker, gid); got != "requested" {
		t.Errorf("asker status = %q, want requested", got)
	}

	// Only the creator is told, and only the creator can see or answer.
	if n := notificationsOf(t, creator, "group_join_request"); len(n) != 1 || n[0]["group"].(map[string]any)["title"] != "Chess" {
		t.Errorf("creator's join request notifications = %v", n)
	}
	if n := notificationsOf(t, member, "group_join_request"); len(n) != 0 {
		t.Errorf("a plain member was told about a join request: %v", n)
	}
	if status, list := creator.list(listPath); status != http.StatusOK || len(list) != 1 || ids(list)[0] != askerID {
		t.Errorf("creator lists requests = %d %v", status, list)
	}
	accept := fmt.Sprintf("/api/group-requests/%d/accept", requestID)
	decline := fmt.Sprintf("/api/group-requests/%d/decline", requestID)
	for who, c := range map[string]*testClient{"member": member, "asker": asker} {
		if status, _ := c.list(listPath); status != http.StatusForbidden {
			t.Errorf("%s lists requests = %d, want 403", who, status)
		}
		for _, path := range []string{accept, decline} {
			if status, _ := c.do("POST", path, "", nil); status != http.StatusForbidden {
				t.Errorf("%s POST %s = %d, want 403", who, path, status)
			}
		}
	}
	if got := myStatus(t, asker, gid); got != "requested" {
		t.Errorf("after the refused attempts status = %q, want requested", got)
	}

	// Declined: the request and its notification are gone; asking again works.
	if status, _ := creator.do("POST", decline, "", nil); status != http.StatusNoContent {
		t.Fatalf("decline = %d", status)
	}
	if n := notificationsOf(t, creator, "group_join_request"); len(n) != 0 {
		t.Errorf("declined request still in the creator's bell: %v", n)
	}
	if got := myStatus(t, asker, gid); got != "none" {
		t.Errorf("after decline status = %q, want none", got)
	}
	if status, _ := creator.do("POST", decline, "", nil); status != http.StatusNotFound {
		t.Errorf("answering a gone request = %d, want 404", status)
	}

	// Cancelling withdraws it from the creator's bell too.
	asker.do("POST", requestPath, "", nil)
	if status, _ := asker.do("DELETE", requestPath, "", nil); status != http.StatusNoContent {
		t.Errorf("cancel = %d, want 204", status)
	}
	if n := notificationsOf(t, creator, "group_join_request"); len(n) != 0 {
		t.Errorf("cancelled request still in the creator's bell: %v", n)
	}
	if status, _ := asker.do("DELETE", requestPath, "", nil); status != http.StatusNotFound {
		t.Errorf("cancel with nothing pending = %d, want 404", status)
	}

	// Accepted: the asker is a member and is told so.
	_, req = asker.do("POST", requestPath, "", nil)
	if status, _ := creator.do("POST", fmt.Sprintf("/api/group-requests/%v/accept", req["id"]), "", nil); status != http.StatusNoContent {
		t.Fatalf("accept = %d", status)
	}
	if got := myStatus(t, asker, gid); got != "member" {
		t.Errorf("after accept status = %q, want member", got)
	}
	if n := notificationsOf(t, asker, "group_request_accepted"); len(n) != 1 {
		t.Errorf("asker's group_request_accepted notifications = %v", n)
	}
	if n := notificationsOf(t, creator, "group_join_request"); len(n) != 0 {
		t.Errorf("accepted request still in the creator's bell: %v", n)
	}
}

// Joining one way clears the other: an invitation accepted while a join
// request is pending removes the request (and the creator's notification).
func TestJoiningClearsTheOtherWayIn(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	creator, _ := register(t, srv.URL, "creator@example.com", "Creator")
	asker, askerID := register(t, srv.URL, "asker@example.com", "Asker")
	gid := createGroup(t, creator, "Runners")

	asker.do("POST", fmt.Sprintf("/api/groups/%d/request", gid), "", nil)
	inviteID := invite(t, creator, gid, askerID)
	if got := myStatus(t, asker, gid); got != "invited" {
		t.Errorf("status with both pending = %q, want invited", got)
	}
	if status, _ := asker.do("POST", fmt.Sprintf("/api/groups/%d/request", gid), "", nil); status != http.StatusConflict {
		t.Errorf("request while invited = %d, want 409", status)
	}
	asker.do("POST", fmt.Sprintf("/api/group-invites/%d/accept", inviteID), "", nil)
	if status, list := creator.list(fmt.Sprintf("/api/groups/%d/requests", gid)); status != http.StatusOK || len(list) != 0 {
		t.Errorf("requests after joining by invitation = %d %v, want none", status, list)
	}
	if n := notificationsOf(t, creator, "group_join_request"); len(n) != 0 {
		t.Errorf("creator still has the join request notification: %v", n)
	}
}

// Gate 3: event RSVP can be changed and the counts update; every other
// member (and nobody else) is told about a new event.
func TestEventsAndResponses(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	creator, _ := register(t, srv.URL, "creator@example.com", "Creator")
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	ben, benID := register(t, srv.URL, "ben@example.com", "Ben")
	outsider, _ := register(t, srv.URL, "outsider@example.com", "Outsider")
	gid := createGroup(t, creator, "Photo walk")
	joinByInvite(t, creator, ann, gid, annID)
	joinByInvite(t, creator, ben, gid, benID)
	eventsPath := fmt.Sprintf("/api/groups/%d/events", gid)

	// Ben is online: the event notification arrives live.
	conn, _, err := dialWS(t, srv, ben, "http://localhost:3000")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	waitFor(t, func() bool { return app.Hub.Connections(benID) == 1 })

	future := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"title":"","description":"d","event_time":"2099-01-01T10:00:00Z"}`, http.StatusBadRequest},
		{`{"title":"t","description":"","event_time":"2099-01-01T10:00:00Z"}`, http.StatusBadRequest},
		{`{"title":"t","description":"d","event_time":"tomorrow"}`, http.StatusBadRequest},
		{`{"title":"t","description":"d","event_time":"2020-01-01T10:00:00Z"}`, http.StatusBadRequest},
	} {
		if status, body := ann.json("POST", eventsPath, tc.body); status != tc.want {
			t.Errorf("create event %s = %d %v, want %d", tc.body, status, body, tc.want)
		}
	}

	// Ann (not the creator) creates the event, with a +02:00 offset.
	status, ev := ann.json("POST", eventsPath, fmt.Sprintf(`{"title":"Sunrise 🌅","description":"Meet at the pier","event_time":%q}`,
		future.In(time.FixedZone("", 2*3600)).Format(time.RFC3339)))
	if status != http.StatusCreated {
		t.Fatalf("create event = %d %v", status, ev)
	}
	if ev["event_time"] != future.Format(sqliteTime) || ev["going_count"] != float64(0) || ev["my_response"] != nil {
		t.Errorf("created event = %v (want event_time %s in UTC)", ev, future.Format(sqliteTime))
	}
	typ, payload := readEvent(t, conn)
	if typ != "notification" || payload["type"] != "group_event" {
		t.Fatalf("Ben's live event = %s %v", typ, payload)
	}
	if e := payload["event"].(map[string]any); e["title"] != "Sunrise 🌅" || payload["group"].(map[string]any)["id"] != float64(gid) {
		t.Errorf("live notification event/group = %v / %v", payload["event"], payload["group"])
	}
	for who, c := range map[string]*testClient{"creator": creator, "ben": ben} {
		if n := notificationsOf(t, c, "group_event"); len(n) != 1 {
			t.Errorf("%s has %d group_event notifications, want 1", who, len(n))
		}
	}
	for who, c := range map[string]*testClient{"ann (who created it)": ann, "outsider": outsider} {
		if n := notificationsOf(t, c, "group_event"); len(n) != 0 {
			t.Errorf("%s got a group_event notification", who)
		}
	}

	respond := fmt.Sprintf("/api/events/%v/respond", ev["id"])
	counts := func(e map[string]any) string {
		return fmt.Sprintf("%v/%v mine=%v", e["going_count"], e["not_going_count"], e["my_response"])
	}
	steps := []struct {
		c        *testClient
		response string
		want     string
	}{
		{ann, "going", "1/0 mine=going"},
		{ben, "going", "2/0 mine=going"},
		{ben, "not_going", "1/1 mine=not_going"}, // changed
		{creator, "not_going", "1/2 mine=not_going"},
		{ben, "going", "2/1 mine=going"}, // changed back
	}
	for i, s := range steps {
		status, e := s.c.json("POST", respond, fmt.Sprintf(`{"response":%q}`, s.response))
		if status != http.StatusOK || counts(e) != s.want {
			t.Errorf("step %d: respond %s = %d %s, want %s", i+1, s.response, status, counts(e), s.want)
		}
	}
	if status, _ := ann.json("POST", respond, `{"response":"maybe"}`); status != http.StatusBadRequest {
		t.Errorf("respond maybe = %d, want 400", status)
	}
	if status, _ := outsider.json("POST", respond, `{"response":"going"}`); status != http.StatusNotFound {
		t.Errorf("outsider responds = %d, want 404", status)
	}
	// The list shows the same counts, with each viewer's own answer.
	_, list := creator.list(eventsPath)
	if len(list) != 1 || counts(list[0]) != "2/1 mine=not_going" {
		t.Errorf("creator lists events = %v", list)
	}
}

// An event that has already happened can no longer be answered: members get
// a 400, everyone else still gets the 404 they get for any event of a group
// they are not in. Answers given before the event stay as they were.
func TestCannotRespondToAPastEvent(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	creator, _ := register(t, srv.URL, "creator@example.com", "Creator")
	ann, annID := register(t, srv.URL, "ann@example.com", "Ann")
	outsider, _ := register(t, srv.URL, "outsider@example.com", "Outsider")
	gid := createGroup(t, creator, "Book club")
	joinByInvite(t, creator, ann, gid, annID)

	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	status, ev := creator.json("POST", fmt.Sprintf("/api/groups/%d/events", gid),
		fmt.Sprintf(`{"title":"Chapter 1","description":"Read it first","event_time":%q}`, future))
	if status != http.StatusCreated {
		t.Fatalf("create event = %d %v", status, ev)
	}
	respond := fmt.Sprintf("/api/events/%d/respond", int64(ev["id"].(float64)))

	// While it is in the future, answering works.
	if status, body := ann.json("POST", respond, `{"response":"going"}`); status != http.StatusOK || body["going_count"] != float64(1) {
		t.Fatalf("respond to a future event = %d %v, want 200 with 1 going", status, body)
	}

	// The event's time passes (the API refuses to create past events, so the
	// clock is moved in the database).
	if _, err := app.DB.Exec(`UPDATE events SET event_time = datetime('now', '-1 hour') WHERE id = ?`, ev["id"]); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]*testClient{"creator": creator, "ann": ann} {
		status, body := c.json("POST", respond, `{"response":"not_going"}`)
		if status != http.StatusBadRequest || body["error"] != "this event has already happened" {
			t.Errorf("%s responds to a past event = %d %v, want 400 \"this event has already happened\"", name, status, body)
		}
	}
	if status, _ := outsider.json("POST", respond, `{"response":"going"}`); status != http.StatusNotFound {
		t.Errorf("outsider responds to a past event = %d, want 404", status)
	}

	// Ann's earlier answer was not changed by the refused one.
	var response string
	if err := app.DB.QueryRow(`SELECT response FROM event_responses WHERE user_id = ?`, annID).Scan(&response); err != nil || response != "going" {
		t.Errorf("ann's stored answer = %q (%v), want going", response, err)
	}
}
