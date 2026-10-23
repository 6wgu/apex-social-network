package handlers

import "log"

const (
	eventPostCreated    = "post_created"
	eventCommentCreated = "comment_created"
	eventPostLikes      = "post_likes"
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

func (a *App) pushToViewers(postID int64, eventType string, payload any) {
	for _, id := range a.Hub.OnlineUsers() {
		ok, err := a.CanViewPost(id, postID)
		if err != nil {
			log.Printf("live %s for post %d, user %d: %v", eventType, postID, id, err)
			continue
		}
		if ok {
			a.Hub.SendToUser(id, eventType, payload)
		}
	}
}
