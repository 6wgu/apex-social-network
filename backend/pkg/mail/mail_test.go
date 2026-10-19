package mail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsoleModeWhenCredentialsEmpty(t *testing.T) {
	m := New(Config{Host: "smtp.gmail.com", Port: 587, FromName: "Kit"})
	var out string
	m.Log = func(f string, a ...any) { out = f }
	if m.Enabled() {
		t.Fatal("enabled without credentials")
	}
	msg := MessageEmail(Content{Subject: "Hi", Paragraphs: []string{"Body"}})
	msg.To = []string{"a@x.com"}
	if err := m.Send(msg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "console mode") {
		t.Fatalf("no console output: %q", out)
	}
	if m.Check() == nil {
		t.Fatal("Check passed without credentials")
	}
}

func TestSendRefusesIncompleteMessages(t *testing.T) {
	m := New(Config{})
	m.Log = func(string, ...any) {}
	for _, msg := range []Message{{Subject: "x", Text: "y"}, {To: []string{"a@x.com"}, Text: "y"}, {To: []string{"a@x.com"}, Subject: "x"}} {
		if m.Send(msg) == nil {
			t.Fatalf("accepted %+v", msg)
		}
	}
}

func TestBuildMIME(t *testing.T) {
	cfg := Config{User: "me@gmail.com", FromName: "My App"}
	msg := Message{To: []string{"Sara <s@x.com>"}, Cc: []string{"c@x.com"}, Bcc: []string{"hidden@x.com"}, Subject: "Héllo",
		Text: "plain", HTML: "<b>html</b>",
		Attachments: []Attachment{{Filename: "a.pdf", Content: []byte("%PDF")}, {Filename: "logo.png", Content: []byte{1, 2}, CID: "logo"}}}
	raw, err := Build(cfg, msg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{`From: "My App" <me@gmail.com>`, "To: Sara <s@x.com>", "Cc: c@x.com", "=?utf-8?q?H=C3=A9llo?=",
		"multipart/mixed", "multipart/alternative", "text/plain; charset=utf-8", "text/html; charset=utf-8",
		`attachment; filename=a.pdf`, "Content-Id: <logo>", "inline; filename=logo.png"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(s, "hidden@x.com") {
		t.Error("Bcc address leaked into the headers")
	}
}

func TestTemplatesEscape(t *testing.T) {
	m := MessageEmail(Content{Subject: "Welcome", Paragraphs: []string{"<b>hi</b>", "second"}, Button: &Button{Text: "Open", URL: "https://x.y/?a=1&b=2"}})
	if !strings.Contains(m.HTML, "&lt;b&gt;hi&lt;/b&gt;") || !strings.Contains(m.HTML, "https://x.y/?a=1&amp;b=2") {
		t.Fatal("HTML not escaped")
	}
	if m.Text != "<b>hi</b>\n\nsecond\n\nOpen: https://x.y/?a=1&b=2" {
		t.Fatalf("text = %q", m.Text)
	}
	c := CodeEmail("Kit", "", "042137", 10)
	if c.Subject != "042137 is your Kit verification code" || !strings.Contains(c.HTML, "042137") || !strings.Contains(c.Text, "10 minutes") {
		t.Fatalf("code email wrong: %+v", c.Subject)
	}
	w := WelcomeEmail("Kit", "<Sara>")
	if w.Subject != "Welcome to Kit" || !strings.Contains(w.HTML, "&lt;Sara&gt;") || !strings.Contains(w.Text, "Hi <Sara>,") {
		t.Fatalf("welcome email wrong: %+v", w.Subject)
	}
}

func TestFromEnvDefaultsAndPassword(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("MAIL_FROM_NAME", "")
	t.Setenv("SMTP_PASS", "abcd efgh ijkl mnop")
	c := FromEnv()
	if c.Host != "smtp.gmail.com" || c.Port != 587 || c.Pass != "abcdefghijklmnop" || c.OTPLength != 6 || c.OTPTTLMinutes != 10 || c.FromName != "APEX" {
		t.Fatalf("%+v", c)
	}
}

func TestLoadDotEnvWindowsFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env.txt"), []byte("\ufeffEK_W1=me@gmail.com\r\nexport EK_W2=abcd efgh\r\n"), 0o600)
	for _, k := range []string{"EK_W1", "EK_W2"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	LoadDotEnv(dir)
	if got := os.Getenv("EK_W1"); got != "me@gmail.com" {
		t.Errorf("EK_W1 = %q", got)
	}
	if got := os.Getenv("EK_W2"); got != "abcd efgh" {
		t.Errorf("EK_W2 = %q", got)
	}
}

func TestLoadDotEnvComments(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("# top\nEK_A=   # empty with comment\nEK_B=587   # port\nEK_C=\"My # App\"\nEK_D='x y'\nEK_E=plain\n"), 0o600)
	for _, k := range []string{"EK_A", "EK_B", "EK_C", "EK_D", "EK_E"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	LoadDotEnv(filepath.Join(dir, "sub", "dir"))
	want := map[string]string{"EK_A": "", "EK_B": "587", "EK_C": "My # App", "EK_D": "x y", "EK_E": "plain"}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}
