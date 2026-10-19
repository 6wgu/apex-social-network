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
// posts (CanViewPost, profile posts, group posts and the feed) uses this
// same text, so the rule lives in one place and is applied by the
// database, not by filtering in Go:
//   - a group post (group_id set) is visible to the group's members only,
//     whatever its privacy column says; nothing below applies to it
//   - the author always sees their own post
//   - public: every logged-in user if the author's profile is public; only
//     accepted followers if it is private (a private profile shows its
//     posts to followers only, whatever each post's own setting)
//   - almost_private: the author's accepted followers
//   - private: the followers the author chose, while they still follow
//
// SQL constant: never built from user input; every value is a bound parameter.
const postVisibleSQL = `(
	(p.group_id IS NOT NULL AND EXISTS (
		SELECT 1 FROM group_members gm WHERE gm.group_id = p.group_id AND gm.user_id = ?1))
	OR (p.group_id IS NULL AND (
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
			WHERE f.follower_id = ?1 AND f.following_id = p.user_id AND f.status = 'accepted'))))
)`

// post is the JSON shape of a post.
type post struct {
	ID           int64              `json:"id"`
	Author       models.UserSummary `json:"author"`
	Content      string             `json:"content"`
	ImagePath    string             `json:"image_path"`
	Privacy      string             `json:"privacy"`
	GroupID      *int64             `json:"group_id"` // null for a profile post
	CommentCount int                `json:"comment_count"`
	LikeCount    int                `json:"like_count"`
	LikedByMe    bool               `json:"liked_by_me"` // whether the viewer (?1) likes it
	CreatedAt    string             `json:"created_at"`
}

// postColumns selects a post (alias p) with its author (alias u), its
// number of comments and likes, and whether the viewer (bound as ?1, as in
// postVisibleSQL) likes it.
// SQL constant: never built from user input; every value is a bound parameter.
const postColumns = `p.id, p.content, p.image_path, p.privacy, p.group_id, strftime('%Y-%m-%d %H:%M:%S', p.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path,
	(SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id),
	(SELECT COUNT(*) FROM post_likes l WHERE l.post_id = p.id),
	EXISTS (SELECT 1 FROM post_likes l WHERE l.post_id = p.id AND l.user_id = ?1)`

// SQL constant: never built from user input; every value is a bound parameter.
const postFrom = `FROM posts p JOIN users u ON u.id = p.user_id`

func scanPost(row interface{ Scan(...any) error }) (post, error) {
	var p post
	var groupID sql.NullInt64
	a := &p.Author
	err := row.Scan(&p.ID, &p.Content, &p.ImagePath, &p.Privacy, &groupID, &p.CreatedAt,
		&a.ID, &a.FirstName, &a.LastName, &a.Nickname, &a.AvatarPath, &p.CommentCount, &p.LikeCount, &p.LikedByMe)
	if groupID.Valid {
		p.GroupID = &groupID.Int64
	}
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
	content, ok := postContent(w, r)
	if !ok {
		return
	}
	privacy := r.FormValue("privacy")
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

	a.savePost(w, r, newPost{authorID: currentUserID(r), content: content, privacy: privacy, viewers: viewers})
}

// newPost is a validated post, ready to be stored by savePost.
type newPost struct {
	authorID int64
	content  string
	privacy  string
	viewers  []int64 // only for private profile posts
	groupID  *int64  // nil for a profile post
}

// savePost finishes POST /api/posts and POST /api/groups/{id}/posts: it
// stores the uploaded image (if any) and the post, then answers 201 with
// the post. The form must already be parsed and the text checked.
func (a *App) savePost(w http.ResponseWriter, r *http.Request, np newPost) {
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
	if np.content == "" && image == "" {
		writeError(w, http.StatusBadRequest, "a post needs text, an image, or both")
		return
	}

	postID, problem, err := a.storePost(np, image)
	if problem != "" || err != nil {
		a.removeImage(image) // nothing points at the file
		if err != nil {
			serverError(w, err)
		} else {
			writeError(w, http.StatusBadRequest, problem)
		}
		return
	}
	p, err := a.postByID(np.authorID, postID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
	a.pushToViewers(p.ID, eventPostCreated, postCreated{PostID: p.ID, AuthorID: p.Author.ID, GroupID: p.GroupID})
}

// postContent reads and checks the content field of a post form. It writes
// the 400 itself and returns false when the text is too long.
func postContent(w http.ResponseWriter, r *http.Request) (string, bool) {
	content := strings.TrimSpace(r.FormValue("content"))
	if utf8.RuneCountInString(content) > maxPostLen {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("a post must be %d characters or fewer", maxPostLen))
		return "", false
	}
	return content, true
}

// storePost inserts a post and its chosen viewers in one transaction. It
// returns a problem (a 400 message) when a chosen viewer is not an accepted
// follower of the author; that is checked inside the transaction, so it
// cannot change between the check and the insert.
func (a *App) storePost(np newPost, image string) (int64, string, error) {
	tx, err := a.DB.Begin()
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()

	authorID := np.authorID
	for _, id := range np.viewers {
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

	res, err := tx.Exec(`INSERT INTO posts (user_id, content, image_path, privacy, group_id) VALUES (?, ?, ?, ?, ?)`,
		authorID, np.content, image, np.privacy, np.groupID)
	if err != nil {
		return 0, "", err
	}
	postID, err := res.LastInsertId()
	if err != nil {
		return 0, "", err
	}
	for _, id := range np.viewers {
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
// Group posts stay in their group and are not listed on profiles.
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
		WHERE p.user_id = ?2 AND p.group_id IS NULL AND p.id < ?3 AND `+postVisibleSQL+`
		ORDER BY p.id DESC LIMIT ?4`, viewer, ownerID, before, postsPerPage)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

// canViewUpload decides GET /uploads/{name} for viewerID. A file that
// belongs to a post, or to a comment on a post, follows that post's
// privacy; any other stored image (an avatar) may be seen by every
// logged-in user.
func (a *App) canViewUpload(viewerID int64, path string) (bool, error) {
	var postID int64
	err := a.DB.QueryRow(`
		SELECT id FROM posts WHERE image_path = ?1
		UNION ALL
		SELECT post_id FROM comments WHERE image_path = ?1
		LIMIT 1`, path).Scan(&postID)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return a.CanViewPost(viewerID, postID)
}

// feed handles GET /api/feed?before={postId}: every profile post the
// viewer may see (their own included), newest first, 20 per page. The
// privacy rule is part of the query (postVisibleSQL), so hidden posts never
// leave the database. Group posts are shown in their group only.
func (a *App) feed(w http.ResponseWriter, r *http.Request) {
	before, ok := beforeParam(w, r)
	if !ok {
		return
	}
	posts, err := a.queryPosts(`SELECT `+postColumns+` `+postFrom+`
		WHERE p.id < ?2 AND p.group_id IS NULL AND `+postVisibleSQL+`
		ORDER BY p.id DESC LIMIT ?3`, currentUserID(r), before, postsPerPage)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}
