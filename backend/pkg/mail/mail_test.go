package mail

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	env := "\xef\xbb\xbf# a comment\n" +
		"MAILTEST_PLAIN = plain value\n" +
		"export MAILTEST_EXPORT=exported\n" +
		"MAILTEST_QUOTED=\"has # inside\"\n" +
		"MAILTEST_SINGLE='single'\n" +
		"MAILTEST_INLINE=value # trailing comment\n" +
		"MAILTEST_EMPTY=# only a comment\n" +
		"MAILTEST_KEEP=from file\n" +
		"not a setting\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	// A variable that is already set wins over the file.
	t.Setenv("MAILTEST_KEEP", "from environment")
	keys := []string{"MAILTEST_PLAIN", "MAILTEST_EXPORT", "MAILTEST_QUOTED", "MAILTEST_SINGLE", "MAILTEST_INLINE", "MAILTEST_EMPTY"}
	t.Cleanup(func() {
		for _, k := range keys {
			os.Unsetenv(k)
		}
	})

	// It is found from a folder below the one that holds it.
	sub := filepath.Join(dir, "a", "b")
	os.MkdirAll(sub, 0o755)
	if got := LoadDotEnv(sub); got != filepath.Join(dir, ".env") {
		t.Fatalf("LoadDotEnv = %q, want the .env two folders up", got)
	}
	for k, want := range map[string]string{
		"MAILTEST_PLAIN":  "plain value",
		"MAILTEST_EXPORT": "exported",
		"MAILTEST_QUOTED": "has # inside",
		"MAILTEST_SINGLE": "single",
		"MAILTEST_INLINE": "value",
		"MAILTEST_EMPTY":  "",
		"MAILTEST_KEEP":   "from environment",
	} {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestFromEnvDefaults(t *testing.T) {
	for _, k := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_SECURE", "SMTP_USER", "MAIL_FROM_NAME",
		"OTP_LENGTH", "OTP_TTL_MINUTES", "OTP_MAX_ATTEMPTS", "OTP_RESEND_SECONDS"} {
		t.Setenv(k, "")
	}
	t.Setenv("SMTP_PASS", "abcd efgh ijkl mnop") // Google shows App Passwords with spaces
	t.Setenv("SMTP_PORT", "not a number")

	got := FromEnv()
	want := Config{Host: "smtp.gmail.com", Port: 587, Pass: "abcdefghijklmnop", FromName: "APEX",
		OTPLength: 6, OTPTTLMinutes: 10, OTPMaxAttempts: 5, OTPResendSeconds: 60}
	if got != want {
		t.Fatalf("FromEnv = %+v\nwant      %+v", got, want)
	}
}

func TestConsoleModeSend(t *testing.T) {
	var logged []string
	m := New(Config{FromName: "APEX"})
	m.Log = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	if m.Enabled() {
		t.Fatal("a mailer without credentials is enabled")
	}

	msg := CodeEmail("APEX", "Ada", "123456", 10)
	msg.To = []string{"ada@example.com"}
	if err := m.Send(msg); err != nil {
		t.Fatal(err)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "ada@example.com") || !strings.Contains(logged[0], "123456") {
		t.Fatalf("console mode logged %q, want the address and the code", logged)
	}
}

// The reset code is in the body only: with SMTP the mailer logs the subject.
func TestResetCodeEmail(t *testing.T) {
	msg := ResetCodeEmail("APEX", "Ada", "123456", 10)
	if strings.Contains(msg.Subject, "123456") {
		t.Errorf("subject %q carries the code", msg.Subject)
	}
	if !strings.Contains(msg.Text, "Your APEX password reset code is 123456.") || !strings.Contains(msg.Text, "10 minutes") {
		t.Errorf("text = %q", msg.Text)
	}
	if !strings.Contains(msg.HTML, "123456") || !strings.Contains(msg.HTML, "reset your password") || strings.Contains(msg.HTML, "verify your email") {
		t.Error("the HTML card does not show the code for a password reset")
	}
	if !strings.Contains(CodeEmail("APEX", "Ada", "123456", 10).HTML, "verify your email") {
		t.Error("the verification card lost its wording")
	}
	if got := PasswordChangedEmail("APEX", "Ada").Subject; got != "Your APEX password was changed" {
		t.Errorf("password changed subject = %q", got)
	}
}

func TestSendChecksTheMessage(t *testing.T) {
	m := New(Config{})
	m.Log = func(string, ...any) {}
	for name, msg := range map[string]Message{
		"no To":      {Subject: "s", Text: "t"},
		"no Subject": {To: []string{"a@example.com"}, Text: "t"},
		"no body":    {To: []string{"a@example.com"}, Subject: "s"},
	} {
		if err := m.Send(msg); err == nil {
			t.Errorf("%s: Send succeeded", name)
		}
	}
}

func TestBuild(t *testing.T) {
	cfg := Config{User: "app@example.com", FromName: "APEX"}
	msg := CodeEmail("APEX", "Ada", "123456", 10)
	msg.To = []string{"ada@example.com"}
	msg.Subject = "Café " + msg.Subject // non-ASCII must be encoded
	raw, err := Build(cfg, msg)
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("not a valid email: %v", err)
	}
	h := parsed.Header
	if from, err := h.AddressList("From"); err != nil || from[0].Name != "APEX" || from[0].Address != "app@example.com" {
		t.Errorf("From = %q (%v)", h.Get("From"), err)
	}
	if h.Get("To") != "ada@example.com" {
		t.Errorf("To = %q", h.Get("To"))
	}
	if subject, err := new(mime.WordDecoder).DecodeHeader(h.Get("Subject")); err != nil || subject != msg.Subject {
		t.Errorf("Subject decodes to %q (%v), want %q", subject, err, msg.Subject)
	}
	if !strings.HasSuffix(h.Get("Message-ID"), "@example.com>") {
		t.Errorf("Message-ID = %q", h.Get("Message-ID"))
	}

	// mixed > related (the HTML and its inline logo) > alternative (text and HTML).
	types := partTypes(t, h.Get("Content-Type"), parsed.Body)
	want := []string{"multipart/related", "multipart/alternative", "text/plain", "text/html", "image/png"}
	if strings.Join(types, " ") != strings.Join(want, " ") {
		t.Fatalf("parts = %v, want %v", types, want)
	}
}

// partTypes lists the media types of every part under a multipart body, depth first.
func partTypes(t *testing.T, contentType string, body io.Reader) []string {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	r := multipart.NewReader(body, params["boundary"])
	for {
		p, err := r.NextPart()
		if err == io.EOF {
			return types
		}
		if err != nil {
			t.Fatal(err)
		}
		typ, _, _ := mime.ParseMediaType(p.Header.Get("Content-Type"))
		types = append(types, typ)
		if strings.HasPrefix(typ, "multipart/") {
			types = append(types, partTypes(t, p.Header.Get("Content-Type"), p)...)
		}
	}
}
