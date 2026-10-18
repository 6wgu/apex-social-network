package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"social-network/pkg/mail"
)

func main() {
	to := flag.String("to", "", "address to send the test email to (default: SMTP_USER)")
	code := flag.Bool("otp", false, "send a sample verification code email")
	flag.Parse()

	if path := mail.LoadDotEnv("."); path != "" {
		fmt.Println("Loaded", path)
	}
	cfg := mail.FromEnv()
	m := mail.New(cfg)
	if !m.Enabled() {
		fmt.Println("SMTP_USER and SMTP_PASS are empty in .env: fill them in first.")
		os.Exit(1)
	}
	fmt.Printf("Logging in to %s:%d as %s...\n", cfg.Host, cfg.Port, cfg.User)
	if err := m.Check(); err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	fmt.Println("Login OK.")

	target := *to
	if target == "" {
		target = cfg.User
	}
	var msg mail.Message
	if *code {
		msg = mail.CodeEmail(cfg.FromName, "", "123456", cfg.OTPTTLMinutes)
	} else {
		msg = mail.MessageEmail(mail.Content{Subject: cfg.FromName + " test email", Greeting: "Hello,",
			Paragraphs: []string{"If you can read this, email sending works."}})
	}
	msg.To = []string{target}
	if err := m.Send(msg); err != nil {
		log.Fatalf("Send failed: %v", err)
	}
	fmt.Println("Test email sent to", target)
}
