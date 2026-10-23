package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
)

const (
	eventPostDeleted    = "post_deleted"
	eventCommentDeleted = "comment_deleted"
)

type postDeleted struct {
	PostID int64 `json:"post_id"`
}

type commentDeleted struct {
	PostID    int64 `json:"post_id"`
	CommentID int64 `json:"comment_id"`
}

func (a *App) broadcast(eventType string, payload any) {
	for _, id := range a.Hub.OnlineUsers() {
		a.Hub.SendToUser(id, eventType, payload)
	}
}

func pathID(w http.ResponseWriter, r *http.Request, name, what string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, what+" not found")
		return 0, false
	}
	return id, true
}

func (a *App) deletePost(w http.ResponseWriter, r *http.Request) {
	postID, ok := pathID(w, r, "id", "post")
	if !ok {
		return
	}
	me := currentUserID(r)

	var authorID int64
	var image string
	err := a.DB.QueryRow(`SELECT user_id, image_path FROM posts WHERE id = ?`, postID).Scan(&authorID, &image)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if authorID != me {
		canView, err := a.CanViewPost(me, postID)
		if err != nil {
			serverError(w, err)
			return
		}
		if !canView {
			writeError(w, http.StatusNotFound, "post not found")
			return
		}
		writeError(w, http.StatusForbidden, "you can only delete your own posts")
		return
	}

	images := []string{image}
	rows, err := a.DB.Query(`SELECT image_path FROM comments WHERE post_id = ? AND image_path <> ''`, postID)
	if err != nil {
		serverError(w, err)
		return
	}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			serverError(w, err)
			return
		}
		images = append(images, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}

	if _, err := a.DB.Exec(`DELETE FROM posts WHERE id = ? AND user_id = ?`, postID, me); err != nil {
		serverError(w, err)
		return
	}
	for _, p := range images {
		a.removeImage(p)
	}
	w.WriteHeader(http.StatusNoContent)
	a.broadcast(eventPostDeleted, postDeleted{PostID: postID})
}

func (a *App) deleteComment(w http.ResponseWriter, r *http.Request) {
	postID, ok := a.visiblePostID(w, r)
	if !ok {
		return
	}
	commentID, ok := pathID(w, r, "commentId", "comment")
	if !ok {
		return
	}
	me := currentUserID(r)

	var commentAuthor, postAuthor int64
	var image string
	err := a.DB.QueryRow(`SELECT c.user_id, c.image_path, p.user_id
		FROM comments c JOIN posts p ON p.id = c.post_id
		WHERE c.id = ? AND c.post_id = ?`, commentID, postID).Scan(&commentAuthor, &image, &postAuthor)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if me != commentAuthor && me != postAuthor {
		writeError(w, http.StatusForbidden, "you can only delete your own comments")
		return
	}
	if _, err := a.DB.Exec(`DELETE FROM comments WHERE id = ?`, commentID); err != nil {
		serverError(w, err)
		return
	}
	a.removeImage(image)
	w.WriteHeader(http.StatusNoContent)
	a.pushToViewers(postID, eventCommentDeleted, commentDeleted{PostID: postID, CommentID: commentID})
}
