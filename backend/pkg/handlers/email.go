package handlers

import (
	"log"

	"social-network/pkg/mail"
	"social-network/pkg/models"
)

func (a *App) sendWelcome(u models.User) {
	if a.Mailer == nil {
		return
	}
	msg := mail.WelcomeEmail(a.Mailer.FromName(), u.FirstName)
	msg.To = []string{u.Email}
	go func() {
		if err := a.Mailer.Send(msg); err != nil {
			log.Printf("welcome email to user %d: %v", u.ID, err)
		}
	}()
}
