package mail

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"path/filepath"
	"strings"
	"time"
)

type Attachment struct {
	Filename    string
	Content     []byte
	ContentType string
	CID         string
}

type Message struct {
	To, Cc, Bcc []string
	ReplyTo     string
	FromName    string
	Subject     string
	Text, HTML  string
	Attachments []Attachment
}

type Mailer struct {
	cfg Config
	Log func(format string, args ...any)
}

func New(cfg Config) *Mailer { return &Mailer{cfg: cfg, Log: log.Printf} }

func (m *Mailer) Enabled() bool { return m.cfg.User != "" && m.cfg.Pass != "" }

func (m *Mailer) FromName() string { return m.cfg.FromName }

func (m *Mailer) Config() Config { return m.cfg }

func (m *Mailer) Send(msg Message) error {
	if len(msg.To) == 0 {
		return errors.New(`send: "To" is required`)
	}
	if msg.Subject == "" {
		return errors.New(`send: "Subject" is required`)
	}
	if msg.Text == "" && msg.HTML == "" {
		return errors.New(`send: give "Text", "HTML" or both`)
	}
	if !m.Enabled() {
		var files []string
		for _, a := range msg.Attachments {
			files = append(files, a.Filename)
		}
		extra := ""
		if len(files) > 0 {
			extra = " + " + strings.Join(files, ", ")
		}
		m.Log("[console mode] email to %s: %q%s\n%s", strings.Join(msg.To, ", "), msg.Subject, extra, msg.Text)
		return nil
	}
	raw, err := Build(m.cfg, msg)
	if err != nil {
		return err
	}
	var rcpt []string
	for _, list := range [][]string{msg.To, msg.Cc, msg.Bcc} {
		for _, a := range list {
			addr, err := mail.ParseAddress(a)
			if err != nil {
				return fmt.Errorf("bad address %q: %w", a, err)
			}
			rcpt = append(rcpt, addr.Address)
		}
	}
	return m.withClient(func(c *smtp.Client) error {
		if err := c.Mail(m.cfg.User); err != nil {
			return err
		}
		for _, r := range rcpt {
			if err := c.Rcpt(r); err != nil {
				return err
			}
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write(raw); err != nil {
			return err
		}
		return w.Close()
	})
}

func (m *Mailer) Check() error {
	if !m.Enabled() {
		return errors.New("SMTP_USER and SMTP_PASS are empty in .env")
	}
	return m.withClient(func(*smtp.Client) error { return nil })
}

func (m *Mailer) withClient(fn func(*smtp.Client) error) error {
	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprint(m.cfg.Port))
	tlsCfg := &tls.Config{ServerName: m.cfg.Host}
	var conn net.Conn
	var err error
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	if m.cfg.Secure {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if !m.cfg.Secure {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("the server does not offer STARTTLS; refusing to send the password unencrypted")
		}
		if err := c.StartTLS(tlsCfg); err != nil {
			return err
		}
	}
	if err := c.Auth(smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)); err != nil {
		return err
	}
	if err := fn(c); err != nil {
		return err
	}
	return c.Quit()
}

func Build(cfg Config, msg Message) ([]byte, error) {
	var buf bytes.Buffer
	name := msg.FromName
	if name == "" {
		name = cfg.FromName
	}
	h := func(k, v string) { fmt.Fprintf(&buf, "%s: %s\r\n", k, v) }
	h("From", (&mail.Address{Name: name, Address: cfg.User}).String())
	h("To", strings.Join(msg.To, ", "))
	if len(msg.Cc) > 0 {
		h("Cc", strings.Join(msg.Cc, ", "))
	}
	if msg.ReplyTo != "" {
		h("Reply-To", msg.ReplyTo)
	}
	h("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("Message-ID", fmt.Sprintf("<%s@%s>", randomID(), domainOf(cfg.User)))
	h("MIME-Version", "1.0")

	outer := multipart.NewWriter(&buf)
	h("Content-Type", fmt.Sprintf("multipart/mixed; boundary=%q", outer.Boundary()))
	buf.WriteString("\r\n")

	var alt bytes.Buffer
	altW := multipart.NewWriter(&alt)
	for _, part := range []struct{ typ, body string }{{"text/plain", msg.Text}, {"text/html", msg.HTML}} {
		if part.body == "" {
			continue
		}
		w, err := altW.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.typ + "; charset=utf-8"},
			"Content-Transfer-Encoding": {"base64"},
		})
		if err != nil {
			return nil, err
		}
		writeBase64(w, []byte(part.body))
	}
	altW.Close()
	w, err := outer.CreatePart(textproto.MIMEHeader{"Content-Type": {fmt.Sprintf("multipart/alternative; boundary=%q", altW.Boundary())}})
	if err != nil {
		return nil, err
	}
	w.Write(alt.Bytes())

	for _, a := range msg.Attachments {
		typ := a.ContentType
		if typ == "" {
			typ = mime.TypeByExtension(filepath.Ext(a.Filename))
		}
		if typ == "" {
			typ = "application/octet-stream"
		}
		hdr := textproto.MIMEHeader{
			"Content-Type":              {mime.FormatMediaType(typ, map[string]string{"name": a.Filename})},
			"Content-Transfer-Encoding": {"base64"},
		}
		if a.CID != "" {
			hdr.Set("Content-ID", "<"+a.CID+">")
			hdr.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": a.Filename}))
		} else {
			hdr.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": a.Filename}))
		}
		w, err := outer.CreatePart(hdr)
		if err != nil {
			return nil, err
		}
		writeBase64(w, a.Content)
	}
	outer.Close()
	return buf.Bytes(), nil
}

func writeBase64(w interface{ Write([]byte) (int, error) }, data []byte) {
	s := base64.StdEncoding.EncodeToString(data)
	for len(s) > 76 {
		w.Write([]byte(s[:76] + "\r\n"))
		s = s[76:]
	}
	w.Write([]byte(s + "\r\n"))
}

func randomID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func domainOf(addr string) string {
	if _, d, ok := strings.Cut(addr, "@"); ok {
		return d
	}
	return "localhost"
}
