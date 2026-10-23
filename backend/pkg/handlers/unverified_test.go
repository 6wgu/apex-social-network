package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"social-network/pkg/db/sqlite"
	"social-network/pkg/mail"
	"social-network/pkg/otp"
	"social-network/pkg/ws"
)

const migrationsForTests = "file://../db/migrations/sqlite"

const strongPassword = "Correct-Horse1"

type testEnv struct {
	app   *App
	url   string
	clock *testClock
	codes *codeStore

	mu       sync.Mutex
	mails    []string
	slowMail atomic.Int64
}

type codeStore struct {
	otp.Store
	pause atomic.Int64
}

func (s *codeStore) Get(p otp.Purpose, subject string) (*otp.Record, error) {
	time.Sleep(time.Duration(s.pause.Load()))
	return s.Store.Get(p, subject)
}

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := sqlite.MigrateUp(db, migrationsForTests); err != nil {
		t.Fatal(err)
	}
	uploads := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		t.Fatal(err)
	}

	e := &testEnv{clock: &testClock{now: time.Now()}, codes: &codeStore{Store: otp.SQLStore{DB: db}}}
	mailer := mail.New(mail.Config{FromName: "APEX"})
	mailer.Log = func(format string, args ...any) {
		time.Sleep(time.Duration(e.slowMail.Load()))
		e.mu.Lock()
		defer e.mu.Unlock()
		e.mails = append(e.mails, fmt.Sprintf(format, args...))
	}
	e.app = &App{DB: db, UploadDir: uploads, Hub: ws.NewHub("http://localhost:3000"), Mailer: mailer,
		OTP: otp.New(e.codes, mailer, otp.Options{Now: e.clock.Now}), RequireVerified: true, Now: e.clock.Now}
	srv := httptest.NewServer(e.app.Routes())
	t.Cleanup(srv.Close)
	e.url = srv.URL
	return e
}

var codeMail = regexp.MustCompile(`(?s)^\[console mode\] email to ([^:]+): .*verification code is (\d{6})`)

func (e *testEnv) codeMails() [][2]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out [][2]string
	for _, m := range e.mails {
		if found := codeMail.FindStringSubmatch(m); found != nil {
			out = append(out, [2]string{found[1], found[2]})
		}
	}
	return out
}

func (e *testEnv) lastCode(t *testing.T) (to, code string) {
	t.Helper()
	all := e.codeMails()
	if len(all) == 0 {
		t.Fatal("no code email was sent")
	}
	last := all[len(all)-1]
	return last[0], last[1]
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (e *testEnv) client(t *testing.T) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client{t: t, base: e.url, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path, contentType string, body io.Reader) (int, map[string]any) {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var out map[string]any
	json.Unmarshal(raw, &out)
	return res.StatusCode, out
}

func (c *client) json(method, path, body string) (int, map[string]any) {
	return c.do(method, path, "application/json", strings.NewReader(body))
}

func (e *testEnv) register(t *testing.T, email string, avatar []byte) (*client, int64) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fields := map[string]string{"email": email, "password": strongPassword,
		"first_name": "Ada", "last_name": "Lovelace", "date_of_birth": "1990-12-10"}
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if avatar != nil {
		fw, _ := mw.CreateFormFile("avatar", "me.png")
		fw.Write(avatar)
	}
	mw.Close()
	c := e.client(t)
	status, user := c.do("POST", "/api/register", mw.FormDataContentType(), &buf)
	if status != http.StatusCreated {
		t.Fatalf("register %s = %d %v", email, status, user)
	}
	return c, int64(user["id"].(float64))
}

func smallPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (e *testEnv) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := e.app.DB.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func (e *testEnv) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.app.DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *testEnv) emailOf(t *testing.T, id int64) string {
	t.Helper()
	var email string
	if err := e.app.DB.QueryRow(`SELECT email FROM users WHERE id = ?`, id).Scan(&email); err != nil {
		t.Fatal(err)
	}
	return email
}

func (e *testEnv) markVerified(t *testing.T, id int64) {
	t.Helper()
	e.exec(t, `UPDATE users SET is_verified = 1 WHERE id = ?`, id)
}

func (e *testEnv) avatarFile(t *testing.T, id int64) string {
	t.Helper()
	var path string
	if err := e.app.DB.QueryRow(`SELECT avatar_path FROM users WHERE id = ?`, id).Scan(&path); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(e.app.UploadDir, strings.TrimPrefix(path, "/uploads/"))
	if path == "" || !exists(file) {
		t.Fatalf("user %d has no avatar file (avatar_path %q)", id, path)
	}
	return file
}

func (e *testEnv) setCreated(t *testing.T, id int64, at time.Time) {
	t.Helper()
	e.exec(t, `UPDATE users SET created_at = ? WHERE id = ?`, at.UTC().Format(time.DateTime), id)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestChangeEmailWhileUnverified(t *testing.T) {
	e := newTestEnv(t)
	c, id := e.register(t, "ada@exmaple.com", nil)

	status, user := c.json("PATCH", "/api/me/email", `{"email":"  Ada@Example.com "}`)
	if status != http.StatusOK || user["email"] != "ada@example.com" || user["is_verified"] != false || user["id"] != float64(id) {
		t.Fatalf("change email = %d %v, want 200 with the new address", status, user)
	}
	if got := e.emailOf(t, id); got != "ada@example.com" {
		t.Errorf("stored email = %q, want ada@example.com", got)
	}
	to, code := e.lastCode(t)
	if to != "ada@example.com" {
		t.Errorf("new code sent to %q, want the new address", to)
	}

	e.register(t, "ada@exmaple.com", nil)

	status, user = c.json("POST", "/api/otp/verify", `{"code":"`+code+`"}`)
	if status != http.StatusOK || user["is_verified"] != true || user["email"] != "ada@example.com" {
		t.Fatalf("verify with the new code = %d %v, want 200 verified", status, user)
	}
}

func TestChangeEmailVoidsTheOldCodeAndKeepsTheCooldown(t *testing.T) {
	e := newTestEnv(t)
	c, id := e.register(t, "old@example.com", nil)
	if status, body := c.json("POST", "/api/otp/send", `{}`); status != http.StatusOK {
		t.Fatalf("send = %d %v", status, body)
	}
	_, oldCode := e.lastCode(t)

	status, body := c.json("PATCH", "/api/me/email", `{"email":"new@example.com"}`)
	if status != http.StatusTooManyRequests || body["retry_after"] == nil {
		t.Fatalf("change during the cooldown = %d %v, want 429 with retry_after", status, body)
	}
	if got := e.emailOf(t, id); got != "old@example.com" {
		t.Errorf("email after the 429 = %q, want it unchanged", got)
	}
	if n := len(e.codeMails()); n != 1 {
		t.Errorf("%d code emails after the 429, want 1", n)
	}

	e.clock.Add(time.Minute)
	if status, body := c.json("PATCH", "/api/me/email", `{"email":"new@example.com"}`); status != http.StatusOK {
		t.Fatalf("change after the cooldown = %d %v, want 200", status, body)
	}
	to, newCode := e.lastCode(t)
	if to != "new@example.com" {
		t.Fatalf("new code sent to %q, want new@example.com", to)
	}

	if oldCode != newCode {
		status, body = c.json("POST", "/api/otp/verify", `{"code":"`+oldCode+`"}`)
		if status != http.StatusBadRequest || body["reason"] != otp.Wrong {
			t.Fatalf("verify with the old code = %d %v, want 400 wrong", status, body)
		}
	}
	if status, _ := c.json("PATCH", "/api/me/email", `{"email":"third@example.com"}`); status != http.StatusTooManyRequests {
		t.Errorf("second change right away = %d, want 429", status)
	}
	status, body = c.json("POST", "/api/otp/verify", `{"code":"`+newCode+`"}`)
	if status != http.StatusOK || body["is_verified"] != true || body["email"] != "new@example.com" {
		t.Fatalf("verify with the new code = %d %v, want 200 verified", status, body)
	}
}

func TestCodeMatchesTheEmailUnderRacingRequests(t *testing.T) {
	e := newTestEnv(t)
	c, id := e.register(t, "race@example.com", nil)
	e.codes.pause.Store(int64(20 * time.Millisecond))
	send := func(method, path, body string) {
		req, err := http.NewRequest(method, e.url+path, strings.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if res, err := c.http.Do(req); err == nil {
			res.Body.Close()
		}
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			send("PATCH", "/api/me/email", fmt.Sprintf(`{"email":"race%d@example.com"}`, i))
		}()
		go func() {
			defer wg.Done()
			send("POST", "/api/otp/send", `{}`)
		}()
	}
	wg.Wait()

	mails := e.codeMails()
	if len(mails) != 1 {
		t.Fatalf("%d code emails, want 1: %v", len(mails), mails)
	}
	if got := e.emailOf(t, id); mails[0][0] != got {
		t.Fatalf("the code went to %s, but the account's email is %s", mails[0][0], got)
	}
	if status, body := c.json("POST", "/api/otp/verify", `{"code":"`+mails[0][1]+`"}`); status != http.StatusOK {
		t.Fatalf("verify with the only code = %d %v, want 200", status, body)
	}
}

func TestChangeEmailWhenVerifiedIs403(t *testing.T) {
	e := newTestEnv(t)
	c, id := e.register(t, "done@example.com", nil)
	e.markVerified(t, id)

	if status, body := c.json("PATCH", "/api/me/email", `{"email":"other@example.com"}`); status != http.StatusForbidden {
		t.Fatalf("change a verified email = %d %v, want 403", status, body)
	}
	if got := e.emailOf(t, id); got != "done@example.com" {
		t.Errorf("email = %q, want it unchanged", got)
	}
	if n := len(e.codeMails()); n != 0 {
		t.Errorf("%d code emails, want none", n)
	}
	if status, _ := e.client(t).json("PATCH", "/api/me/email", `{"email":"other@example.com"}`); status != http.StatusUnauthorized {
		t.Errorf("change without a session = %d, want 401", status)
	}
}

func TestChangeEmailToATakenEmailIs409(t *testing.T) {
	e := newTestEnv(t)
	e.register(t, "taken@example.com", nil)
	c, id := e.register(t, "me@example.com", nil)

	for _, email := range []string{"taken@example.com", "TAKEN@Example.com"} {
		status, body := c.json("PATCH", "/api/me/email", `{"email":"`+email+`"}`)
		fields, _ := body["fields"].(map[string]any)
		if status != http.StatusConflict || fields["email"] != "email already registered" {
			t.Errorf("change to %s = %d %v, want 409 with fields.email", email, status, body)
		}
	}
	if got := e.emailOf(t, id); got != "me@example.com" {
		t.Errorf("email = %q, want it unchanged", got)
	}
	if n := len(e.codeMails()); n != 0 {
		t.Errorf("%d code emails, want none: a taken address must not get a code", n)
	}
}

func TestChangeEmailChecksTheAddress(t *testing.T) {
	e := newTestEnv(t)
	c, id := e.register(t, "me@example.com", nil)

	for body, want := range map[string]string{
		`{"email":""}`:               "email is required",
		`{"email":"a@b"}`:            "email must be a valid address like name@example.com",
		`{"email":"a b@x.com"}`:      "email must be a valid address like name@example.com",
		`{"email":"a@x..com"}`:       "email must be a valid address like name@example.com",
		`{"email":".a@x.com"}`:       "email must be a valid address like name@example.com",
		`{"email":"ME@Example.com"}`: "that is already your email",
	} {
		status, got := c.json("PATCH", "/api/me/email", body)
		fields, _ := got["fields"].(map[string]any)
		if status != http.StatusBadRequest || fields["email"] != want || got["error"] != want {
			t.Errorf("%s = %d %v, want 400 %q", body, status, got, want)
		}
	}
	for _, body := range []string{`not json`, `{"email":"x@example.com","extra":1}`} {
		if status, _ := c.json("PATCH", "/api/me/email", body); status != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", body, status)
		}
	}
	if got := e.emailOf(t, id); got != "me@example.com" {
		t.Errorf("email = %q, want it unchanged", got)
	}
	if n := len(e.codeMails()); n != 0 {
		t.Errorf("%d code emails, want none", n)
	}
}

func TestStartOverDeletesOnlyAnUnverifiedAccount(t *testing.T) {
	e := newTestEnv(t)
	c, id := e.register(t, "oops@exmaple.com", smallPNG(t))
	avatar := e.avatarFile(t, id)
	if status, body := c.json("POST", "/api/otp/send", `{}`); status != http.StatusOK {
		t.Fatalf("send = %d %v", status, body)
	}
	_, otherID := e.register(t, "other@example.com", nil)

	if status, body := c.do("DELETE", "/api/me", "", nil); status != http.StatusNoContent {
		t.Fatalf("start over = %d %v, want 204", status, body)
	}
	for query, arg := range map[string]any{
		`SELECT COUNT(*) FROM users WHERE id = ?`:          id,
		`SELECT COUNT(*) FROM sessions WHERE user_id = ?`:  id,
		`SELECT COUNT(*) FROM otp_codes WHERE subject = ?`: otpSubject(id),
	} {
		if n := e.count(t, query, arg); n != 0 {
			t.Errorf("%s with %v = %d after starting over, want 0", query, arg, n)
		}
	}
	if exists(avatar) {
		t.Error("the avatar file is still on disk")
	}
	base, _ := url.Parse(e.url)
	if cookies := c.http.Jar.Cookies(base); len(cookies) != 0 {
		t.Errorf("cookies after starting over = %v, want none", cookies)
	}
	if status, _ := c.json("GET", "/api/me", ""); status != http.StatusUnauthorized {
		t.Errorf("GET /api/me after starting over = %d, want 401", status)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM users WHERE id = ?`, otherID); n != 1 {
		t.Error("another unverified account was deleted too")
	}
	e.register(t, "oops@exmaple.com", nil)

	v, vid := e.register(t, "kept@example.com", smallPNG(t))
	e.markVerified(t, vid)
	vAvatar := e.avatarFile(t, vid)
	if status, body := v.do("DELETE", "/api/me", "", nil); status != http.StatusForbidden {
		t.Fatalf("start over when verified = %d %v, want 403", status, body)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM users WHERE id = ?`, vid); n != 1 || !exists(vAvatar) {
		t.Error("the verified account or its avatar was deleted")
	}
	if status, _ := v.json("GET", "/api/me", ""); status != http.StatusOK {
		t.Errorf("GET /api/me after the 403 = %d, want 200", status)
	}

	if status, _ := e.client(t).do("DELETE", "/api/me", "", nil); status != http.StatusUnauthorized {
		t.Errorf("start over without a session = %d, want 401", status)
	}
}

func TestDeleteAbandonedAccounts(t *testing.T) {
	e := newTestEnv(t)
	now := time.Now()
	abandoned, abandonedID := e.register(t, "typo@exmaple.com", smallPNG(t))
	_, recentID := e.register(t, "recent@example.com", nil)
	_, verifiedID := e.register(t, "verified@example.com", smallPNG(t))
	e.markVerified(t, verifiedID)
	e.setCreated(t, abandonedID, now.Add(-UnverifiedAccountTTL-time.Minute))
	e.setCreated(t, recentID, now.Add(-UnverifiedAccountTTL+time.Minute))
	e.setCreated(t, verifiedID, now.Add(-30*UnverifiedAccountTTL))
	abandonedAvatar := e.avatarFile(t, abandonedID)
	verifiedAvatar := e.avatarFile(t, verifiedID)

	if n, err := e.app.DeleteAbandonedAccounts(now); err != nil || n != 1 {
		t.Fatalf("DeleteAbandonedAccounts = %d, %v; want 1", n, err)
	}
	if e.count(t, `SELECT COUNT(*) FROM users WHERE id = ?`, abandonedID) != 0 ||
		e.count(t, `SELECT COUNT(*) FROM sessions WHERE user_id = ?`, abandonedID) != 0 || exists(abandonedAvatar) {
		t.Error("the abandoned account, its sessions or its avatar file are still there")
	}
	if status, _ := abandoned.json("GET", "/api/me", ""); status != http.StatusUnauthorized {
		t.Errorf("GET /api/me with the abandoned account's cookie = %d, want 401", status)
	}
	if e.count(t, `SELECT COUNT(*) FROM users WHERE id IN (?, ?)`, recentID, verifiedID) != 2 || !exists(verifiedAvatar) {
		t.Error("a recent or a verified account was deleted")
	}
	e.register(t, "typo@exmaple.com", nil)

	if n, err := e.app.DeleteAbandonedAccounts(now.Add(2 * time.Minute)); err != nil || n != 1 {
		t.Fatalf("second pass = %d, %v; want 1", n, err)
	}
	if e.count(t, `SELECT COUNT(*) FROM users WHERE id = ?`, recentID) != 0 {
		t.Error("the account that passed the limit is still there")
	}
	if e.count(t, `SELECT COUNT(*) FROM users WHERE id = ?`, verifiedID) != 1 || !exists(verifiedAvatar) {
		t.Error("the verified account or its avatar was deleted")
	}
}

func TestDeleteAbandonedAccountRemovesItsImages(t *testing.T) {
	e := newTestEnv(t)
	_, oldID := e.register(t, "old@example.com", nil)
	_, friendID := e.register(t, "friend@example.com", nil)
	e.markVerified(t, friendID)
	e.setCreated(t, oldID, time.Now().Add(-90*24*time.Hour))
	for _, name := range []string{"post.png", "comment.png", "friend.png"} {
		if err := os.WriteFile(filepath.Join(e.app.UploadDir, name), smallPNG(t), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e.exec(t, `INSERT INTO posts (id, user_id, image_path, privacy) VALUES (100, ?, '/uploads/post.png', 'public')`, oldID)
	e.exec(t, `INSERT INTO comments (post_id, user_id, image_path) VALUES (100, ?, '/uploads/comment.png')`, friendID)
	e.exec(t, `INSERT INTO posts (user_id, image_path, privacy) VALUES (?, '/uploads/friend.png', 'public')`, friendID)

	if n, err := e.app.DeleteAbandonedAccounts(time.Now()); err != nil || n != 1 {
		t.Fatalf("DeleteAbandonedAccounts = %d, %v; want 1", n, err)
	}
	for _, name := range []string{"post.png", "comment.png"} {
		if exists(filepath.Join(e.app.UploadDir, name)) {
			t.Errorf("%s is still on disk", name)
		}
	}
	if !exists(filepath.Join(e.app.UploadDir, "friend.png")) || e.count(t, `SELECT COUNT(*) FROM posts WHERE user_id = ?`, friendID) != 1 {
		t.Error("the verified friend's own post or image was removed")
	}
}

func TestCleanUpRunsRightAway(t *testing.T) {
	e := newTestEnv(t)
	_, id := e.register(t, "gone@example.com", nil)
	e.setCreated(t, id, time.Now().Add(-UnverifiedAccountTTL-time.Hour))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e.app.CleanUpAbandonedAccounts(ctx)
	if e.count(t, `SELECT COUNT(*) FROM users WHERE id = ?`, id) != 0 {
		t.Error("the first pass, at startup, did not delete the abandoned account")
	}
}
