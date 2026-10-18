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
	IsVerified   bool   `json:"is_verified"`
	CreatedAt    string `json:"created_at"`
}

// UserSummary is the public face of a user: what lists, search results and
// private profiles show. It has no email, birth date or about text.
type UserSummary struct {
	ID         int64  `json:"id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Nickname   string `json:"nickname"`
	AvatarPath string `json:"avatar_path"`
}
