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
	privacyPublic        = "public"
	privacyAlmostPrivate = "almost_private"
	privacyPrivate       = "private"

	maxPostLen   = 5000
	postsPerPage = 20
)

// postVisibleSQL is the post privacy rule as a SQL condition on a posts row
// aliased p, with the viewer's id bound as ?1. Every query that returns
// posts (CanViewPost, profile posts, and the feed) uses this same text, so
// the rule lives in one place and is applied by the database, not by
// filtering in Go:
//   - the author always sees their own post
//   - public: every logged-in user if the author's profile is public; only
//     accepted followers if it is private (a private profile shows its
//     posts to followers only, whatever each post's own setting)
//   - almost_private: the author's accepted followers
//   - private: the followers the author chose, while they still follow
const postVisibleSQL = `(
	p.user_id = ?1
	OR (p.privacy = 'public' AND (
		EXISTS (SELECT 1 FROM users author WHERE author.id = p.user_id AND author.is_public = 1)
		OR EXISTS (
		SELECT 1 FROM follows f
		WHERE f.follower_id = ?1 AND f.following_id = p.user_id AND f.status = 'accepted')))
	OR (p.privacy = 'almost_private' AND EXISTS (
		SELECT 1 FROM follows f
		WHERE f.follower_id = ?1 AND f.following_id = p.user_id AND f.status = 'accepted'))
	OR (p.privacy = 'private' AND EXISTS (
		SELECT 1 FROM post_viewers pv WHERE pv.post_id = p.id AND pv.user_id = ?1)
		AND EXISTS (
		SELECT 1 FROM follows f
		WHERE f.follower_id = ?1 AND f.following_id = p.user_id AND f.status = 'accepted'))
)`

// post is the JSON shape of a post.
type post struct {
	ID           int64              `json:"id"`
	Author       models.UserSummary `json:"author"`
	Content      string             `json:"content"`
	ImagePath    string             `json:"image_path"`
	Privacy      string             `json:"privacy"`
	CommentCount int                `json:"comment_count"`
	CreatedAt    string             `json:"created_at"`
}

// postColumns selects a post (alias p) with its author (alias u).
const postColumns = `p.id, p.content, p.image_path, p.privacy, strftime('%Y-%m-%d %H:%M:%S', p.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path`

const postFrom = `FROM posts p JOIN users u ON u.id = p.user_id`

func scanPost(row interface{ Scan(...any) error }) (post, error) {
	var p post
	a := &p.Author
	err := row.Scan(&p.ID, &p.Content, &p.ImagePath, &p.Privacy, &p.CreatedAt,
		&a.ID, &a.FirstName, &a.LastName, &a.Nickname, &a.AvatarPath)
	return p, err
}

// queryPosts runs a SELECT postColumns query and returns the posts (never nil).
func (a *App) queryPosts(query string, args ...any) ([]post, error) {
	rows, err := a.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := []post{}
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

// CanViewPost reports whether viewerID may see postID. It returns
// sql.ErrNoRows when the post does not exist.
func (a *App) CanViewPost(viewerID, postID int64) (bool, error) {
	var allowed bool
	err := a.DB.QueryRow(`SELECT `+postVisibleSQL+` FROM posts p WHERE p.id = ?2`, viewerID, postID).Scan(&allowed)
	return allowed, err
}

// postByID loads a post for viewerID, or sql.ErrNoRows when it does not exist
// or viewerID may not see it: the two cases look the same to the client.
func (a *App) postByID(viewerID, postID int64) (post, error) {
	return scanPost(a.DB.QueryRow(`SELECT `+postColumns+` `+postFrom+`
		WHERE p.id = ?2 AND `+postVisibleSQL, viewerID, postID))
}

// beforeParam reads ?before={id} for paging newest-first lists. With no
// value it returns the largest id, i.e. "from the newest".
func beforeParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	v := r.URL.Query().Get("before")
	if v == "" {
		return 1<<63 - 1, true
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "before must be an id")
		return 0, false
	}
	return id, true
}

// createPost handles POST /api/posts (multipart): content, privacy,
// viewer_ids (repeated, only for private) and an optional image.
func (a *App) createPost(w http.ResponseWriter, r *http.Request) {
	defer removeFormFiles(r)
	if !parseForm(w, r) {
		return
	}
	me := currentUserID(r)
	content := strings.TrimSpace(r.FormValue("content"))
	privacy := r.FormValue("privacy")

	if utf8.RuneCountInString(content) > maxPostLen {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("a post must be %d characters or fewer", maxPostLen))
		return
	}
	switch privacy {
	case privacyPublic, privacyAlmostPrivate, privacyPrivate:
	default:
		writeError(w, http.StatusBadRequest, "privacy must be public, almost_private or private")
		return
	}

	viewers, problem := parseIDs(r.Form["viewer_ids"])
	switch {
	case problem != "":
		writeError(w, http.StatusBadRequest, problem)
		return
	case privacy == privacyPrivate && len(viewers) == 0:
		writeError(w, http.StatusBadRequest, "choose at least one follower for a private post")
		return
	case privacy != privacyPrivate && len(viewers) > 0:
		writeError(w, http.StatusBadRequest, "viewer_ids is only for private posts")
		return
	}

	// The file is written before the transaction starts, so a slow upload
	// never holds the database write lock. From here on, every failure
	// deletes it again.
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
		writeError(w, http.StatusBadRequest, "a post needs text, an image, or both")
		return
	}

	postID, problem, err := a.storePost(me, content, image, privacy, viewers)
	if problem != "" || err != nil {
		a.removeImage(image) // nothing points at the file
		if err != nil {
			serverError(w, err)
		} else {
			writeError(w, http.StatusBadRequest, problem)
		}
		return
	}
	p, err := a.postByID(me, postID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// storePost inserts a post and its chosen viewers in one transaction. It
// returns a problem (a 400 message) when a chosen viewer is not an accepted
// follower of the author; that is checked inside the transaction, so it
// cannot change between the check and the insert.
func (a *App) storePost(authorID int64, content, image, privacy string, viewers []int64) (int64, string, error) {
	tx, err := a.DB.Begin()
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()

	for _, id := range viewers {
		var follows bool
		err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = ? AND following_id = ? AND status = 'accepted')`,
			id, authorID).Scan(&follows)
		if err != nil {
			return 0, "", err
		}
		if !follows {
			return 0, fmt.Sprintf("user %d is not one of your followers", id), nil
		}
	}

	res, err := tx.Exec(`INSERT INTO posts (user_id, content, image_path, privacy) VALUES (?, ?, ?, ?)`,
		authorID, content, image, privacy)
	if err != nil {
		return 0, "", err
	}
	postID, err := res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	for _, id := range viewers {
		if _, err := tx.Exec(`INSERT INTO post_viewers (post_id, user_id) VALUES (?, ?)`, postID, id); err != nil {
			return 0, "", err
		}
	}
	return postID, "", tx.Commit()
}

// parseIDs reads repeated id fields, dropping duplicates.
func parseIDs(values []string) ([]int64, string) {
	seen := map[int64]bool{}
	var ids []int64
	for _, v := range values {
		for _, part := range strings.Split(v, ",") { // "1,2" works as well as repeated fields
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id <= 0 {
				return nil, "viewer_ids must be user ids"
			}
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids, ""
}

// getPost handles GET /api/posts/{id}. A post the viewer may not see is a
// 404, exactly like one that does not exist, so its existence is not revealed.
func (a *App) getPost(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || postID <= 0 {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	p, err := a.postByID(currentUserID(r), postID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "post not found")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// userPosts handles GET /api/users/{id}/posts?before={postId}: the profile's
// posts that the viewer may see, newest first, 20 per page. A private
// profile shows nothing (403) to anyone but its owner and accepted followers.
func (a *App) userPosts(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.userIDFromPath(w, r, "id")
	if !ok {
		return
	}
	before, ok := beforeParam(w, r)
	if !ok {
		return
	}
	viewer := currentUserID(r)
	canView, err := a.CanViewProfile(viewer, ownerID)
	if err != nil {
		serverError(w, err)
		return
	}
	if !canView {
		writeError(w, http.StatusForbidden, "this profile is private")
		return
	}
	posts, err := a.queryPosts(`SELECT `+postColumns+` `+postFrom+`
		WHERE p.user_id = ?2 AND p.id < ?3 AND `+postVisibleSQL+`
		ORDER BY p.id DESC LIMIT ?4`, viewer, ownerID, before, postsPerPage)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

// canViewUpload decides GET /uploads/{name} for viewerID. A file that
// belongs to a post follows that post's privacy; any other stored image
// (an avatar) may be seen by every logged-in user.
func (a *App) canViewUpload(viewerID int64, path string) (bool, error) {
	var postID int64
	err := a.DB.QueryRow(`SELECT id FROM posts WHERE image_path = ?`, path).Scan(&postID)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return a.CanViewPost(viewerID, postID)
}
