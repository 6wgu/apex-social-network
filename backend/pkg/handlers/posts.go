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

type post struct {
	ID           int64              `json:"id"`
	Author       models.UserSummary `json:"author"`
	Content      string             `json:"content"`
	ImagePath    string             `json:"image_path"`
	Privacy      string             `json:"privacy"`
	GroupID      *int64             `json:"group_id"`
	CommentCount int                `json:"comment_count"`
	LikeCount    int                `json:"like_count"`
	LikedByMe    bool               `json:"liked_by_me"`
	CreatedAt    string             `json:"created_at"`
}

const postColumns = `p.id, p.content, p.image_path, p.privacy, p.group_id, strftime('%Y-%m-%d %H:%M:%S', p.created_at),
	u.id, u.first_name, u.last_name, u.nickname, u.avatar_path,
	(SELECT COUNT(*) FROM comments c WHERE c.post_id = p.id),
	(SELECT COUNT(*) FROM post_likes l WHERE l.post_id = p.id),
	EXISTS (SELECT 1 FROM post_likes l WHERE l.post_id = p.id AND l.user_id = ?1)`

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

func (a *App) CanViewPost(viewerID, postID int64) (bool, error) {
	var allowed bool
	err := a.DB.QueryRow(`SELECT `+postVisibleSQL+` FROM posts p WHERE p.id = ?2`, viewerID, postID).Scan(&allowed)
	return allowed, err
}

func (a *App) postByID(viewerID, postID int64) (post, error) {
	return scanPost(a.DB.QueryRow(`SELECT `+postColumns+` `+postFrom+`
		WHERE p.id = ?2 AND `+postVisibleSQL, viewerID, postID))
}

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

type newPost struct {
	authorID int64
	content  string
	privacy  string
	viewers  []int64
	groupID  *int64
}

func (a *App) savePost(w http.ResponseWriter, r *http.Request, np newPost) {
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
		a.removeImage(image)
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

func postContent(w http.ResponseWriter, r *http.Request) (string, bool) {
	content := strings.TrimSpace(r.FormValue("content"))
	if utf8.RuneCountInString(content) > maxPostLen {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("a post must be %d characters or fewer", maxPostLen))
		return "", false
	}
	return content, true
}

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

func parseIDs(values []string) ([]int64, string) {
	seen := map[int64]bool{}
	var ids []int64
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
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
