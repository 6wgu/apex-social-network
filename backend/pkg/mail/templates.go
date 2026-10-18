package mail

import (
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

var cardTmpl = template.Must(template.New("card").Parse(`<!DOCTYPE html>
<html><head><meta charset="UTF-8"></head>
<body style="margin:0;padding:0;background:#edf2ef;font-family:Georgia,'Times New Roman',serif;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background:#edf2ef;padding:40px 0;"><tr><td align="center">
    <table width="480" cellpadding="0" cellspacing="0" style="background:#fbfcfa;border:1px solid #d5dfda;border-radius:12px;overflow:hidden;">
      <tr><td style="background:#293b38;padding:28px 40px;text-align:center;">{{if .LogoCID}}<img src="cid:{{.LogoCID}}" alt="" height="40" style="display:block;margin:0 auto 12px;">{{end}}
        <h1 style="margin:0;color:#f4f0e8;font-size:20px;font-weight:400;letter-spacing:.5px;">{{.Title}}</h1>
      </td></tr>
      <tr><td style="padding:32px 40px;color:#263632;font-size:15px;line-height:1.6;">
        {{if .Code}}<p style="margin:0 0 24px;text-align:center;color:#6b7973;">{{if .Name}}Hi <strong style="color:#263632;">{{.Name}}</strong>, use{{else}}Use{{end}} the code below to verify your email.</p>
        <div style="text-align:center;margin:0 0 24px;"><span style="display:inline-block;background:#edf2ef;border:2px solid #3f756d;border-radius:10px;padding:18px 32px;font-size:38px;font-weight:700;letter-spacing:12px;color:#3f756d;font-family:'Courier New',monospace;">{{.Code}}</span></div>
        <p style="margin:0;text-align:center;color:#6b7973;font-size:13px;">This code expires in <strong style="color:#263632;">{{.TTL}} minutes</strong>.</p>
        {{else}}{{if .Greeting}}<p style="margin:0 0 16px;">{{.Greeting}}</p>{{end}}{{range .Paragraphs}}<p style="margin:0 0 16px;">{{.}}</p>{{end}}
        {{with .Button}}<p style="margin:24px 0 8px;text-align:center;"><a href="{{.URL}}" style="display:inline-block;background:#3f756d;color:#ffffff;text-decoration:none;font-weight:700;padding:12px 28px;border-radius:8px;">{{.Text}}</a></p>{{end}}{{end}}
      </td></tr>
      <tr><td style="border-top:1px solid #d5dfda;padding:18px 40px;text-align:center;">
        <p style="color:#6b7973;font-size:12px;margin:0;">{{.Footer}}</p>
      </td></tr>
    </table>
  </td></tr></table>
</body></html>`))

type cardData struct {
	Content
	Code, Name string
	TTL        int
}

func render(d cardData) string {
	if d.Title == "" {
		d.Title = d.Subject
	}
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
	return Message{Subject: c.Subject, Text: strings.Join(parts, "\n\n"), HTML: render(cardData{Content: c})}
}

func CodeEmail(appName, name, code string, ttlMinutes int) Message {
	text := fmt.Sprintf("Your %s verification code is %s.\nIt expires in %d minutes.\n\nIf you did not ask for this, you can ignore this email.", appName, code, ttlMinutes)
	if name != "" {
		text = "Hi " + name + ",\n\n" + text
	}
	c := Content{Subject: fmt.Sprintf("%s is your %s verification code", code, appName), Title: appName + " verification",
		Footer: "If you did not ask for this, you can ignore this email."}
	return Message{Subject: c.Subject, Text: text, HTML: render(cardData{Content: c, Code: code, Name: name, TTL: ttlMinutes})}
}

func WelcomeEmail(appName, name string) Message {
	return MessageEmail(Content{
		Subject:    "Welcome to " + appName,
		Greeting:   "Hi " + name + ",",
		Paragraphs: []string{"Your " + appName + " account is ready.", "Verify your email address from the app to secure your account."},
		Footer:     "You received this email because an account was created with this address.",
	})
}
