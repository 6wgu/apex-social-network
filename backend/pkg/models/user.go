// Package models holds the structs shared between the database and the JSON API.
package models

// User mirrors the users table. PasswordHash is never sent to clients.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	DateOfBirth  string `json:"date_of_birth"`
	AvatarPath   string `json:"avatar_path"`
	Nickname     string `json:"nickname"`
	AboutMe      string `json:"about_me"`
	IsPublic     bool   `json:"is_public"`
	CreatedAt    string `json:"created_at"`
}
