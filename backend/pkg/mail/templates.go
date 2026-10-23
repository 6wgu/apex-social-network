package mail

import (
	_ "embed"
	"fmt"
	"html/template"
	"strings"
)

type Button struct{ Text, URL string }

type Content struct {
	Subject    string
	Title      string
	Greeting   string
	Paragraphs []string
	Button     *Button
	Footer     string
	LogoCID    string
}

const (
	logoCID = "apex-logo"
	fonts   = `'Space Grotesk','DM Sans','Segoe UI',Helvetica,sans-serif`
)

//go:embed apex-logo.png
var logoPNG []byte

func withLogo(m Message) Message {
	m.Attachments = append(m.Attachments, Attachment{Filename: "apex-logo.png", Content: logoPNG, ContentType: "image/png", CID: logoCID})
	return m
}

var cardTmpl = template.Must(template.New("card").Parse(`<!DOCTYPE html>
<html><head><meta charset="UTF-8"><meta name="color-scheme" content="dark"><meta name="supported-color-schemes" content="dark"></head>
<body style="margin:0;padding:0;background:#090C12;font-family:` + fonts + `;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#090C12;padding:40px 12px;"><tr><td align="center">
    <table role="presentation" width="480" cellpadding="0" cellspacing="0" style="max-width:480px;width:100%;background:#121722;border:1px solid #303847;border-radius:16px;overflow:hidden;">
      <tr><td style="height:4px;background:#FF3045;font-size:0;line-height:0;">&nbsp;</td></tr>
      <tr><td style="background:#0B0E14;padding:28px 40px 24px;text-align:center;border-bottom:1px solid #303847;">
        {{if .LogoCID}}<img src="cid:{{.LogoCID}}" alt="APEX" width="168" height="37" style="display:block;margin:0 auto 14px;border:0;">{{end}}
        <h1 style="margin:0;color:#F5F7FA;font-size:20px;font-weight:700;letter-spacing:.3px;">{{.Title}}</h1>
      </td></tr>
      <tr><td style="padding:32px 40px;color:#F5F7FA;font-size:15px;line-height:1.6;">
        {{if .Code}}<p style="margin:0 0 24px;text-align:center;color:#9BA6B8;">{{if .Name}}Hi <strong style="color:#F5F7FA;">{{.Name}}</strong>, use{{else}}Use{{end}} the code below to {{.Action}}.</p>
        <div style="text-align:center;margin:0 0 24px;"><span style="display:inline-block;background:#1B2230;border:2px solid #FF3045;border-radius:12px;padding:18px 28px;font-size:36px;font-weight:700;letter-spacing:12px;color:#F5F7FA;font-family:'Space Grotesk','Courier New',monospace;">{{.Code}}</span></div>
        <p style="margin:0;text-align:center;color:#9BA6B8;font-size:13px;">This code expires in <strong style="color:#27D8E8;">{{.TTL}} minutes</strong>.</p>
        {{else}}{{if .Greeting}}<p style="margin:0 0 16px;">{{.Greeting}}</p>{{end}}{{range .Paragraphs}}<p style="margin:0 0 16px;color:#9BA6B8;">{{.}}</p>{{end}}
        {{with .Button}}<p style="margin:24px 0 8px;text-align:center;"><a href="{{.URL}}" style="display:inline-block;background:#FF3045;color:#FFFFFF;text-decoration:none;font-weight:700;padding:12px 28px;border-radius:10px;">{{.Text}}</a></p>{{end}}{{end}}
      </td></tr>
      <tr><td style="border-top:1px solid #303847;padding:18px 40px;text-align:center;background:#0B0E14;">
        <p style="color:#9BA6B8;font-size:12px;margin:0 0 6px;">{{.Footer}}</p>
        <p style="color:#5f6a7d;font-size:11px;margin:0;letter-spacing:.12em;text-transform:uppercase;">APEX &middot; Connect at full speed</p>
      </td></tr>
    </table>
  </td></tr></table>
</body></html>`))

type cardData struct {
	Content
	Code, Name string
	TTL        int
	Action     string
}

func render(d cardData) string {
	if d.Title == "" {
		d.Title = d.Subject
	}
	d.LogoCID = logoCID
	if d.Footer == "" {
		d.Footer = "If you did not expect this email, you can ignore it."
	}
	var b strings.Builder
	cardTmpl.Execute(&b, d)
	return b.String()
}

func MessageEmail(c Content) Message {
	parts := []string{}
	if c.Greeting != "" {
		parts = append(parts, c.Greeting)
	}
	parts = append(parts, c.Paragraphs...)
	if c.Button != nil {
		parts = append(parts, c.Button.Text+": "+c.Button.URL)
	}
	return withLogo(Message{Subject: c.Subject, Text: strings.Join(parts, "\n\n"), HTML: render(cardData{Content: c})})
}

func CodeEmail(appName, name, code string, ttlMinutes int) Message {
	text := fmt.Sprintf("Your %s verification code is %s.\nIt expires in %d minutes.\n\nIf you did not ask for this, you can ignore this email.", appName, code, ttlMinutes)
	if name != "" {
		text = "Hi " + name + ",\n\n" + text
	}
	c := Content{Subject: fmt.Sprintf("%s is your %s verification code", code, appName), Title: "Verify your email",
		Footer: "If you did not ask for this, you can ignore this email."}
	return withLogo(Message{Subject: c.Subject, Text: text, HTML: render(cardData{Content: c, Code: code, Name: name, TTL: ttlMinutes, Action: "verify your email"})})
}

func ResetCodeEmail(appName, name, code string, ttlMinutes int) Message {
	text := fmt.Sprintf("Your %s password reset code is %s.\nIt expires in %d minutes.\n\nIf you didn't ask for this, ignore this email.", appName, code, ttlMinutes)
	if name != "" {
		text = "Hi " + name + ",\n\n" + text
	}
	c := Content{Subject: fmt.Sprintf("Your %s password reset code", appName), Title: "Reset your password",
		Footer: "If you didn't ask for this, ignore this email. Your password stays the same."}
	return withLogo(Message{Subject: c.Subject, Text: text, HTML: render(cardData{Content: c, Code: code, Name: name, TTL: ttlMinutes, Action: "reset your password"})})
}

func PasswordChangedEmail(appName, name string) Message {
	return MessageEmail(Content{
		Subject:  "Your " + appName + " password was changed",
		Greeting: "Hi " + name + ",",
		Paragraphs: []string{
			"The password of your " + appName + " account was just changed, and every device that was signed in to it has been signed out.",
			"If this wasn't you, use \"Forgot password?\" on the login page right away to choose a new one.",
		},
		Footer: "You received this email because the password of your account was changed.",
	})
}

func WelcomeEmail(appName, name string) Message {
	return MessageEmail(Content{
		Subject:  "Welcome to " + appName,
		Greeting: "Hi " + name + ",",
		Paragraphs: []string{
			"Your " + appName + " account is ready.",
			"One step left: enter the 6-digit code we sent you on the verify page. Once your email is verified, you are on the grid: posts, groups, events and live chat.",
		},
		Footer: "You received this email because an account was created with this address.",
	})
}
