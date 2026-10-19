package handlers

import "net/http"

// likeState is the answer to a like or unlike: the post's new count and
// whether the logged-in user now likes it.
type likeState struct {
	LikeCount int  `json:"like_count"`
	LikedByMe bool `json:"liked_by_me"`
}

// likePost handles POST /api/posts/{id}/like and DELETE /api/posts/{id}/like.
// Liking twice, or unliking a post you never liked, is not an error. A post
// the user may not see is a 404, exactly like one that does not exist; for a
// like that is checked again inside the INSERT, so it cannot change between
// the check and the write.
func (a *App) likePost(like bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, ok := a.visiblePostID(w, r)
		if !ok {
			return
		}
		userID := currentUserID(r)
		var err error
		if like {
			_, err = a.DB.Exec(`INSERT OR IGNORE INTO post_likes (post_id, user_id)
				SELECT p.id, ?1 FROM posts p WHERE p.id = ?2 AND `+postVisibleSQL, userID, postID)
		} else {
			_, err = a.DB.Exec(`DELETE FROM post_likes WHERE post_id = ? AND user_id = ?`, postID, userID)
		}
		if err != nil {
			serverError(w, err)
			return
		}
		var s likeState
		err = a.DB.QueryRow(`SELECT COUNT(*), COALESCE(MAX(user_id = ?1), 0) FROM post_likes WHERE post_id = ?2`,
			userID, postID).Scan(&s.LikeCount, &s.LikedByMe)
		if err != nil {
			serverError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s)
		a.pushToViewers(postID, eventPostLikes, postLikes{PostID: postID, LikeCount: s.LikeCount})
	}
}
