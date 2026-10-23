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

// sendPasswordChanged tells the user, at their address, that their
// password was reset.
func (a *App) sendPasswordChanged(userID int64) {
	if a.Mailer == nil {
		return
	}
	u, err := a.userByID(userID)
	if err != nil {
		log.Printf("password changed email to user %d: %v", userID, err)
		return
	}
	msg := mail.PasswordChangedEmail(a.Mailer.FromName(), u.FirstName)
	msg.To = []string{u.Email}
	a.mailing.Go(func() {
		if err := a.Mailer.Send(msg); err != nil {
			log.Printf("password changed email to user %d: %v", userID, err)
		}
	})
}
