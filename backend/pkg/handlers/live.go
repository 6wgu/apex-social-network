package handlers

import "log"

// Live updates for posts. Each goes only to online users who may see the
// post (the same postVisibleSQL rule as every read), so nothing leaks.
const (
	eventPostCreated    = "post_created"    // payload: postCreated; the page fetches the post itself
	eventCommentCreated = "comment_created" // payload: comment
	eventPostLikes      = "post_likes"      // payload: postLikes
)

type postCreated struct {
	PostID   int64  `json:"post_id"`
	AuthorID int64  `json:"author_id"`
	GroupID  *int64 `json:"group_id"`
}

type postLikes struct {
	PostID    int64 `json:"post_id"`
	LikeCount int   `json:"like_count"`
}

// pushToViewers sends an event about postID to every online user who can
// see that post, the actor's other tabs included.
func (a *App) pushToViewers(postID int64, eventType string, payload any) {
	for _, id := range a.Hub.OnlineUsers() {
		ok, err := a.CanViewPost(id, postID)
		if err != nil {
			log.Printf("live %s for post %d: %v", eventType, postID, err)
			return
		}
		if ok {
			a.Hub.SendToUser(id, eventType, payload)
		}
	}
}
