package handlers

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// testClient is an HTTP client with its own cookie jar, i.e. one browser.
type testClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, base string) *testClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &testClient{t: t, base: base, http: &http.Client{Jar: jar}}
}

// do sends a request and returns the status and the decoded JSON body.
func (c *testClient) do(method, path, contentType string, body io.Reader) (int, map[string]any) {
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

func (c *testClient) get(path string) (int, map[string]any) {
	return c.do(http.MethodGet, path, "", nil)
}

func (c *testClient) json(method, path, body string) (int, map[string]any) {
	return c.do(method, path, "application/json", strings.NewReader(body))
}

// form sends a multipart form; avatar, when not nil, is sent as the "avatar" file.
func (c *testClient) form(method, path string, fields map[string]string, avatar []byte) (int, map[string]any) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	if avatar != nil {
		fw, _ := mw.CreateFormFile("avatar", "whatever.png")
		fw.Write(avatar)
	}
	mw.Close()
	return c.do(method, path, mw.FormDataContentType(), &buf)
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func validUser(email string) map[string]string {
	return map[string]string{
		"email": email, "password": "correct horse", "first_name": "Ada",
		"last_name": "Lovelace", "date_of_birth": "1990-12-10",
	}
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRegisterLoginLogout(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	c := newClient(t, srv.URL)

	if status, _ := c.get("/api/me"); status != http.StatusUnauthorized {
		t.Fatalf("GET /api/me before login = %d, want 401", status)
	}

	status, user := c.form("POST", "/api/register", validUser("Ada@Example.com"), nil)
	if status != http.StatusCreated {
		t.Fatalf("register = %d %v, want 201", status, user)
	}
	if user["email"] != "ada@example.com" || user["is_public"] != true {
		t.Errorf("unexpected user: %v", user)
	}
	if _, leaked := user["password_hash"]; leaked {
		t.Error("response contains password_hash")
	}

	var hash string
	app.DB.QueryRow(`SELECT password_hash FROM users WHERE email = ?`, "ada@example.com").Scan(&hash)
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("correct horse")) != nil {
		t.Errorf("stored password %q is not a bcrypt hash of the password", hash)
	}

	// Registering logs you in.
	if status, me := c.get("/api/me"); status != http.StatusOK || me["first_name"] != "Ada" {
		t.Errorf("GET /api/me after register = %d %v", status, me)
	}

	if status, body := c.form("POST", "/api/register", validUser("ADA@example.com"), nil); status != http.StatusConflict {
		t.Errorf("duplicate email = %d %v, want 409", status, body)
	}

	// Logout kills the session on the server: the old cookie stops working.
	oldCookies := c.http.Jar.Cookies(mustURL(t, srv.URL))
	if status, _ := c.do("POST", "/api/logout", "", nil); status != http.StatusNoContent {
		t.Errorf("logout = %d, want 204", status)
	}
	replay := newClient(t, srv.URL)
	replay.http.Jar.SetCookies(mustURL(t, srv.URL), oldCookies)
	if status, _ := replay.get("/api/me"); status != http.StatusUnauthorized {
		t.Errorf("GET /api/me with the pre-logout cookie = %d, want 401", status)
	}

	if status, _ := c.json("POST", "/api/login", `{"email":"ada@example.com","password":"wrong password"}`); status != http.StatusUnauthorized {
		t.Errorf("login with wrong password = %d, want 401", status)
	}
	if status, _ := c.json("POST", "/api/login", `{"email":"nobody@example.com","password":"correct horse"}`); status != http.StatusUnauthorized {
		t.Errorf("login with unknown email = %d, want 401", status)
	}
	if status, _ := c.json("POST", "/api/login", `{"email":"ada@example.com","password":"correct horse"}`); status != http.StatusOK {
		t.Errorf("login = %d, want 200", status)
	}
	if status, _ := c.get("/api/me"); status != http.StatusOK {
		t.Errorf("GET /api/me after login = %d, want 200", status)
	}
}

func TestRegisterValidation(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	c := newClient(t, srv.URL)

	cases := map[string]func(map[string]string){
		"missing email":      func(f map[string]string) { delete(f, "email") },
		"bad email":          func(f map[string]string) { f["email"] = "not-an-email" },
		"email without tld":  func(f map[string]string) { f["email"] = "ada@localhost" },
		"short password":     func(f map[string]string) { f["password"] = "1234567" },
		"blank first name":   func(f map[string]string) { f["first_name"] = "   " },
		"missing last name":  func(f map[string]string) { delete(f, "last_name") },
		"missing birth date": func(f map[string]string) { delete(f, "date_of_birth") },
		"garbage birth date": func(f map[string]string) { f["date_of_birth"] = "31/12/1990" },
		"impossible date":    func(f map[string]string) { f["date_of_birth"] = "1990-02-30" },
		"future birth date":  func(f map[string]string) { f["date_of_birth"] = "2999-01-01" },
	}
	for name, breakIt := range cases {
		fields := validUser("ok@example.com")
		breakIt(fields)
		status, body := c.form("POST", "/api/register", fields, nil)
		if status != http.StatusBadRequest || body["error"] == nil {
			t.Errorf("%s: got %d %v, want 400 with an error message", name, status, body)
		}
	}
}

func TestRegisterAvatar(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()

	c := newClient(t, srv.URL)
	status, user := c.form("POST", "/api/register", validUser("pic@example.com"), tinyPNG(t))
	if status != http.StatusCreated {
		t.Fatalf("register with avatar = %d %v", status, user)
	}
	path, _ := user["avatar_path"].(string)
	if !strings.HasPrefix(path, "/uploads/") || !strings.HasSuffix(path, ".png") {
		t.Fatalf("avatar_path = %q, want /uploads/<uuid>.png", path)
	}
	res, err := c.http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" {
		t.Errorf("GET %s = %d %s", path, res.StatusCode, res.Header.Get("Content-Type"))
	}

	// Text with an image name, and an image over the size limit.
	other := newClient(t, srv.URL)
	if status, body := other.form("POST", "/api/register", validUser("txt@example.com"), []byte("just some text")); status != http.StatusBadRequest {
		t.Errorf("text file as avatar = %d %v, want 400", status, body)
	}
	big := append(tinyPNG(t), make([]byte, maxImageBytes)...)
	if status, body := other.form("POST", "/api/register", validUser("big@example.com"), big); status != http.StatusBadRequest {
		t.Errorf("oversized avatar = %d %v, want 400", status, body)
	}
	huge := append(tinyPNG(t), make([]byte, 12<<20)...)
	if status, body := other.form("POST", "/api/register", validUser("huge@example.com"), huge); status != http.StatusBadRequest {
		t.Errorf("12 MB avatar = %d %v, want 400", status, body)
	}
	var n int
	app.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	if n != 1 {
		t.Errorf("%d users after three rejected avatars, want 1", n)
	}
	if files, _ := os.ReadDir(app.UploadDir); len(files) != 1 {
		t.Errorf("%d files in the upload dir after rejected avatars, want 1", len(files))
	}
}
