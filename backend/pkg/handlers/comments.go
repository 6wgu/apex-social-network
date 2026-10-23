package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"social-network/pkg/models"
)

const (
	maxCommentLen   = 2000
	commentsPerPage = 50
)

type comment struct {
	ID        int64              `json:"id"`
	PostID    int64              `json:"post_id"`
	Author    models.UserSummary `json:"author"`
	Content   string             `json:"content"`
	ImagePath string             `json:"image_path"`
	CreatedAt string             `json:"created_at"`
}

const commentColumns = `c.id, c.post_id, c.content, c.image_path, strftime('%Y-%m-%d %H:%M:%S', c.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path`

const commentFrom = `FROM comments c JOIN users u ON u.id = c.user_id`

func scanComment(row interface{ Scan(...any) error }) (comment, error) {
	var c comment
	a := &c.Author
	err := row.Scan(&c.ID, &c.PostID, &c.Content, &c.ImagePath, &c.CreatedAt,
		&a.ID, &a.FirstName, &a.LastName, &a.Nickname, &a.AvatarPath)
	return c, err
}

func (a *App) visiblePostID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusNotFound, "post not found")
		return 0, false
	}
	canView, err := a.CanViewPost(currentUserID(r), postID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !canView) {
		writeError(w, http.StatusNotFound, "post not found")
		return 0, false
	}
	if err != nil {
		serverError(w, err)
		return 0, false
	}
	return postID, true
}

func (a *App) listComments(w http.ResponseWriter, r *http.Request) {
	postID, ok := a.visiblePostID(w, r)
	if !ok {
		return
	}
	after := int64(0)
	if v := r.URL.Query().Get("after"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 0 {
			writeError(w, http.StatusBadRequest, "after must be a comment id")
			return
		}
		after = id
	}
	rows, err := a.DB.Query(`SELECT `+commentColumns+` `+commentFrom+`
		WHERE c.post_id = ? AND c.id > ?
		ORDER BY c.id LIMIT ?`, postID, after, commentsPerPage)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	comments := []comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			serverError(w, err)
			return
		}
		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, comments)
}

func (a *App) createComment(w http.ResponseWriter, r *http.Request) {
	defer removeFormFiles(r)
	postID, ok := a.visiblePostID(w, r)
	if !ok {
		return
	}
	if !parseForm(w, r) {
		return
	}
	content := strings.TrimSpace(r.FormValue("content"))
	if utf8.RuneCountInString(content) > maxCommentLen {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("a comment must be %d characters or fewer", maxCommentLen))
		return
	}
	image, err := a.saveImage(r, "image")
	var uploadErr uploadError
	if errors.As(err, &uploadErr) {
		writeError(w, http.StatusBadRequest, "image: "+uploadErr.Error())
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if content == "" && image == "" {
		writeError(w, http.StatusBadRequest, "a comment needs text, an image, or both")
		return
	}

	me := currentUserID(r)
	res, err := a.DB.Exec(`INSERT INTO comments (post_id, user_id, content, image_path)
		SELECT p.id, ?1, ?3, ?4 FROM posts p WHERE p.id = ?2 AND `+postVisibleSQL, me, postID, content, image)
	if err != nil {
		a.removeImage(image)
		serverError(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		a.removeImage(image)
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	id, err := res.LastInsertId()
	if err != nil {
		serverError(w, err)
		return
	}
	c, err := scanComment(a.DB.QueryRow(`SELECT `+commentColumns+` `+commentFrom+` WHERE c.id = ?`, id))
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
	a.pushToViewers(postID, eventCommentCreated, c)
}
