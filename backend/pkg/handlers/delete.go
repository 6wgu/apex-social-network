package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
)

// Live event for a removed post. It carries an id only, so they are sent to every
// online user: the post is already gone, so its audience can no longer be
// worked out, and an id reveals nothing.
const eventPostDeleted = "post_deleted" // payload: postDeleted

type postDeleted struct {
	PostID int64 `json:"post_id"`
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

// deletePost handles DELETE /api/posts/{id}. Only the author can delete a
// post. The post's comments, likes and viewers go with it (ON DELETE
// CASCADE), and so do the image files of the post and of its comments.
// Anyone else gets a 404 for a post they cannot see and a 403 for one they can.
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

	// Collect the comment images before the rows disappear.
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
