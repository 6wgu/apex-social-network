package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/mattn/go-sqlite3"

	"social-network/pkg/models"
)

const (
	followNone      = "none"
	followPending   = "pending"
	followFollowing = "following"
	followSelf      = "self"
)

func apiFollowStatus(dbStatus string) string {
	if dbStatus == "accepted" {
		return followFollowing
	}
	return dbStatus
}

func (a *App) userIDFromPath(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "user not found")
		return 0, false
	}
	var exists bool
	if err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)`, id).Scan(&exists); err != nil {
		serverError(w, err)
		return 0, false
	}
	if !exists {
		writeError(w, http.StatusNotFound, "user not found")
		return 0, false
	}
	return id, true
}

func (a *App) follow(w http.ResponseWriter, r *http.Request) {
	targetID, ok := a.userIDFromPath(w, r, "id")
	if !ok {
		return
	}
	me := currentUserID(r)
	if targetID == me {
		writeError(w, http.StatusBadRequest, "you cannot follow yourself")
		return
	}

	tx, err := a.DB.Begin()
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()

	var isPublic bool
	if err := tx.QueryRow(`SELECT is_public FROM users WHERE id = ?`, targetID).Scan(&isPublic); err != nil {
		serverError(w, err)
		return
	}
	status := "pending"
	if isPublic {
		status = "accepted"
	}
	_, err = tx.Exec(`INSERT INTO follows (follower_id, following_id, status) VALUES (?, ?, ?)`, me, targetID, status)
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
		var existing string
		if err := tx.QueryRow(`SELECT status FROM follows WHERE follower_id = ? AND following_id = ?`, me, targetID).Scan(&existing); err != nil {
			serverError(w, err)
			return
		}
		if existing == "accepted" {
			writeError(w, http.StatusConflict, "you already follow this user")
		} else {
			writeError(w, http.StatusConflict, "you already sent a follow request to this user")
		}
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	var n notifier
	if status == "pending" {
		err = n.add(tx, targetID, me, notifFollowRequest, nil)
	} else {
		err = n.add(tx, targetID, me, notifNewFollower, nil)
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	a.push(&n)
	writeJSON(w, http.StatusOK, map[string]string{"follow_status": apiFollowStatus(status)})
}

func (a *App) unfollow(w http.ResponseWriter, r *http.Request) {
	targetID, ok := a.userIDFromPath(w, r, "id")
	if !ok {
		return
	}
	me := currentUserID(r)
	tx, err := a.DB.Begin()
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()
	var status string
	err = tx.QueryRow(`DELETE FROM follows WHERE follower_id = ? AND following_id = ? RETURNING status`, me, targetID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "you are not following this user and have no pending request to them")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	var n notifier
	if status == "pending" {
		if err := n.remove(tx, targetID, me, notifFollowRequest); err != nil {
			serverError(w, err)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	a.push(&n)
	w.WriteHeader(http.StatusNoContent)
}

type followRequest struct {
	User      models.UserSummary `json:"user"`
	CreatedAt string             `json:"created_at"`
}

func (a *App) listFollowRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := a.DB.Query(`
		SELECT u.id, u.first_name, u.last_name, u.nickname, u.avatar_path,
		       strftime('%Y-%m-%d %H:%M:%S', f.created_at)
		FROM follows f JOIN users u ON u.id = f.follower_id
		WHERE f.following_id = ? AND f.status = 'pending'
		ORDER BY f.created_at, u.id`, currentUserID(r))
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	requests := []followRequest{}
	for rows.Next() {
		var fr followRequest
		u := &fr.User
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.AvatarPath, &fr.CreatedAt); err != nil {
			serverError(w, err)
			return
		}
		requests = append(requests, fr)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (a *App) answerFollowRequest(accept bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requesterID, err := strconv.ParseInt(r.PathValue("userId"), 10, 64)
		if err != nil || requesterID <= 0 {
			writeError(w, http.StatusNotFound, "follow request not found")
			return
		}
		me := currentUserID(r)
		query := `DELETE FROM follows WHERE follower_id = ? AND following_id = ? AND status = 'pending'`
		if accept {
			query = `UPDATE follows SET status = 'accepted' WHERE follower_id = ? AND following_id = ? AND status = 'pending'`
		}
		tx, err := a.DB.Begin()
		if err != nil {
			serverError(w, err)
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(query, requesterID, me)
		if err != nil {
			serverError(w, err)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			writeError(w, http.StatusNotFound, "follow request not found")
			return
		}
		var n notifier
		err = n.remove(tx, me, requesterID, notifFollowRequest)
		if err == nil && accept {
			err = n.add(tx, requesterID, me, notifFollowAccepted, nil)
		}
		if err != nil {
			serverError(w, err)
			return
		}
		if err := tx.Commit(); err != nil {
			serverError(w, err)
			return
		}
		a.push(&n)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (a *App) listFollows(followers bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID, ok := a.userIDFromPath(w, r, "id")
		if !ok {
			return
		}
		canView, err := a.CanViewProfile(currentUserID(r), ownerID)
		if err != nil {
			serverError(w, err)
			return
		}
		if !canView {
			writeError(w, http.StatusForbidden, "this profile is private")
			return
		}
		query := `
			SELECT u.id, u.first_name, u.last_name, u.nickname, u.avatar_path
			FROM follows f JOIN users u ON u.id = f.follower_id
			WHERE f.following_id = ? AND f.status = 'accepted'
			ORDER BY u.first_name, u.last_name, u.id`
		if !followers {
			query = `
			SELECT u.id, u.first_name, u.last_name, u.nickname, u.avatar_path
			FROM follows f JOIN users u ON u.id = f.following_id
			WHERE f.follower_id = ? AND f.status = 'accepted'
			ORDER BY u.first_name, u.last_name, u.id`
		}
		users, err := a.userSummaries(query, ownerID)
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, users)
	}
}

func (a *App) userSummaries(query string, args ...any) ([]models.UserSummary, error) {
	rows, err := a.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []models.UserSummary{}
	for rows.Next() {
		var u models.UserSummary
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName, &u.Nickname, &u.AvatarPath); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func acceptPendingRequests(tx *sql.Tx, n *notifier, userID int64) error {
	rows, err := tx.Query(`UPDATE follows SET status = 'accepted' WHERE following_id = ? AND status = 'pending' RETURNING follower_id`, userID)
	if err != nil {
		return err
	}
	var requesters []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		requesters = append(requesters, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, requesterID := range requesters {
		if err := n.remove(tx, userID, requesterID, notifFollowRequest); err != nil {
			return err
		}
		if err := n.add(tx, requesterID, userID, notifFollowAccepted, nil); err != nil {
			return err
		}
	}
	return nil
}
