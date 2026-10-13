# API contract

The frontend and backend are built in parallel against this contract. Change it
only by pull request, and tell both sides.

## Conventions

- The browser calls everything on the frontend origin (`http://localhost:3000`). Next.js rewrites `/api/*`, `/uploads/*` and `/ws` to the backend (`:8080`).
- JSON uses `snake_case`. Times are UTC strings in the format SQLite returns: `"2026-10-13 12:00:00"`.
- Every error response has the same shape: `{"error": "human readable message"}`.
- Auth uses the `session_id` cookie: HttpOnly, SameSite=Lax, 7-day expiry, plus `Secure` when the backend runs with `COOKIE_SECURE=1` (off by default, because the app is served over plain `http://localhost`). Every route except `GET /api/health`, `POST /api/register`, `POST /api/login` and `POST /api/logout` returns **401** without a valid session (that includes `GET /ws`). That includes `/uploads/*`: images are only served to logged-in users, and post images only to users who may see the post.
- Requests with an image are `multipart/form-data`; all other bodies are JSON.
- Images are JPEG, PNG or GIF, max 5 MB, checked by file bytes. They are saved as `<uuid>.jpg|.png|.gif` (the extension comes from the detected type, not the file name) and served from `image_path`, e.g. `/uploads/3f2a….png`. `GET /uploads/` is a `404`: there is no listing.
- Status codes: `200` ok, `201` created, `204` no body, `400` bad input, `401` not logged in, `403` logged in but not allowed, `404` not found **or hidden from you**, `405` wrong method for an existing path (with an `Allow` header), `409` duplicate.

### Shared shapes

```jsonc
// UserSummary: used in lists, post authors, chat contacts
{ "id": 1, "first_name": "Ada", "last_name": "Lovelace", "nickname": "ada", "avatar_path": "/uploads/x.png" }

// Post
{
  "id": 10, "author": UserSummary,
  "content": "Hello 👋", "image_path": "",   // either may be empty, never both
  "privacy": "public",            // public | almost_private | private
  "comment_count": 2, "created_at": "2026-10-13 12:00:00"
}

// Comment
{ "id": 5, "post_id": 10, "author": UserSummary, "content": "Nice", "image_path": "", "created_at": "..." }
```

---

## Health (Phase 0)

| Method | Path | Response |
|---|---|---|
| GET | `/api/health` | `200 {"status":"ok","database":"ok"}` |

## Auth (Phase 1)

| Method | Path | Body | Response |
|---|---|---|---|
| POST | `/api/register` | multipart: `email`, `password`, `first_name`, `last_name`, `date_of_birth` (YYYY-MM-DD), optional `nickname`, `about_me`, `avatar` (file) | `201 User` and sets the cookie |
| POST | `/api/login` | `{"email","password"}` | `200 User` and sets the cookie |
| POST | `/api/logout` | none | `204`, always. Public route: deletes the session row if the cookie names one, and always clears the cookie, even when it is expired or unknown |
| GET | `/api/me` | none | `200 User` |

`User` is the full profile without the password:
`{ "id", "email", "first_name", "last_name", "date_of_birth", "avatar_path", "nickname", "about_me", "is_public", "created_at" }`

Validation (all on the server): valid email format, password 8 to 72 characters, a real birth date in the past, and first and last name required (max 50 characters). `nickname` is at most 30 characters and `about_me` at most 500. A duplicate email returns `409 {"error":"email already registered"}`; emails are compared without case and stored in lower case. A bad avatar returns `400` with `avatar: image must be a JPEG, PNG or GIF` or `avatar: image must be 5 MB or smaller`.

Login with an unknown email or a wrong password returns the same `401 {"error":"wrong email or password"}`.

## Profiles and users (Phase 1)

| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/api/users/{id}` | none | `200 Profile` |
| PATCH | `/api/me` | multipart: any of `nickname`, `about_me`, `avatar` (file), `remove_avatar=1` | `200 User`. A field you leave out keeps its value; send `nickname` or `about_me` empty to clear it. `remove_avatar=1` clears the avatar and deletes its file; a new `avatar` replaces the old file. `400` if nothing is sent, if `avatar` and `remove_avatar` are both sent, or if `remove_avatar` is anything but `1` |
| PATCH | `/api/me/privacy` | `{"is_public": false}` | `200 User`. Always changes the logged-in user; there is no way to name another user |
| GET | `/api/users?search=ada` | none | `200 [UserSummary]` (max 20; empty search lists users). Matches first name, last name, full name or nickname, ignoring case |

`Profile` when the viewer is the owner, the profile is public, or the viewer is an accepted follower:

```jsonc
{
  "user": User,                 // email etc. included, never the password
  "can_view": true,
  "follow_status": "none",      // none | pending | following  (viewer -> this user); "self" on own profile
  "follower_count": 3, "following_count": 5   // accepted follows only; pending requests are not counted
}
```

Otherwise (private and not following) only basic info is returned:

```jsonc
{ "user": UserSummary, "can_view": false, "follow_status": "none" | "pending" }
```

## Followers (Phase 2)

A row in `follows` means *follower → following*. Its `status` is `pending` until a private user accepts, and `accepted` after that. A public user is followed with `accepted` straight away. Declining, cancelling and unfollowing delete the row, so the same pair can start over at any time. The API calls the states `none`, `pending`, `following` and `self`.

| Method | Path | Body | Response |
|---|---|---|---|
| POST | `/api/users/{id}/follow` | none | `200 {"follow_status":"following"}` if the target is public, `200 {"follow_status":"pending"}` if private (the target gets a `follow_request` notification if private, `new_follower` if public). `400` for yourself, `404` unknown user, `409 "you already follow this user"` or `409 "you already sent a follow request to this user"` |
| DELETE | `/api/users/{id}/follow` | none | `204`: unfollows, or cancels my pending request. `404 "you are not following this user and have no pending request to them"` if there is nothing to remove, or the user does not exist |
| GET | `/api/follow-requests` | none | `200 [{ "user": UserSummary, "created_at" }]`: pending requests sent **to me**, oldest first |
| POST | `/api/follow-requests/{userId}/accept` | none | `204`: `{userId}` now follows me. `404` if `{userId}` has no pending request to me. Only the person a request was sent to can answer it, because the request is looked up as `{userId}` → logged-in user |
| POST | `/api/follow-requests/{userId}/decline` | none | `204`: deletes the request; the requester may ask again. `404` as for accept |
| GET | `/api/users/{id}/followers` | none | `200 [UserSummary]`: users who follow `{id}` (accepted only), sorted by name. `403 "this profile is private"` if the profile is private and you are neither its owner nor an accepted follower, `404` unknown user |
| GET | `/api/users/{id}/following` | none | `200 [UserSummary]`: users `{id}` follows (accepted only). Same `403` / `404` rule as followers |

`PATCH /api/me/privacy` with `{"is_public": true}` also accepts every pending request to you, in the same transaction.

## Posts and comments (Phase 2)

| Method | Path | Body | Response |
|---|---|---|---|
| POST | `/api/posts` | multipart: `content` (up to 5000 characters), `privacy`, `viewer_ids` (only for `private`: repeat the field once per id, or send `1,2,3`), optional `image` (JPEG/PNG/GIF, 5 MB) | `201 Post`. `400` when: both `content` and `image` are empty; `privacy` is not one of the three; a `private` post has no viewers; a non-private post has `viewer_ids`; a viewer is not one of your **accepted** followers (the message names the id); or the image is invalid |
| GET | `/api/posts/{id}` | none | `200 Post`. `404` if it does not exist **or you may not see it** (the two look the same) |
| GET | `/api/users/{id}/posts?before={postId}` | none | `200 [Post]`: that user's posts that you may see, newest first, 20 per page. `403 "this profile is private"` for a private profile you neither own nor follow, `404` unknown user, `400` bad `before` |
| GET | `/api/feed?before={postId}` | none | (step 5) `200 [Post]`: up to 20, newest first, only posts you may see |
| GET | `/api/posts/{id}/comments` | none | `200 [Comment]` oldest first. `404` if you can't see the post |
| POST | `/api/posts/{id}/comments` | multipart: `content`, optional `image` | `201 Comment`. `404` if you can't see the post |

Who can see a post (`CanViewPost`). The rule is a single SQL condition, `postVisibleSQL` in `pkg/handlers/posts.go`, and every endpoint that returns posts uses it:
- The author always sees their own post.
- `public`: every logged-in user.
- `almost_private`: the author's accepted followers.
- `private`: the followers chosen in `post_viewers`, **while they still follow** the author. Unfollowing takes the access away; following again gives it back.

`GET /api/users/{id}/posts` first applies the profile rule (`CanViewProfile`): a private profile's posts, even its public ones, are listed only to the owner and to accepted followers.

**Images.** `GET /uploads/{name}` checks whether the file belongs to a post. If it does, it applies that post's `CanViewPost`, and someone who may not see the post gets the same `404` as for a missing file. Any other stored image (an avatar) is served to every logged-in user. Post images are sent with `Cache-Control: private, no-cache`.

## Groups and events (Phase 3)

| Method | Path | Body | Response |
|---|---|---|---|
| POST | `/api/groups` | `{"title","description"}` | `201 Group` (the creator becomes a member) |
| GET | `/api/groups` | none | `200 [Group]` (all groups, for browsing) |
| GET | `/api/groups/{id}` | none | `200 Group`, plus `members: [UserSummary]` if you are a member |
| POST | `/api/groups/{id}/invite` | `{"user_id": 7}` | `201` (members only; `409` if already a member or already invited) |
| POST | `/api/group-invites/{inviteId}/accept` | none | `204` (invitee only) |
| POST | `/api/group-invites/{inviteId}/decline` | none | `204` |
| POST | `/api/groups/{id}/request` | none | `201` (non-members; `409` if already requested) |
| GET | `/api/groups/{id}/requests` | none | `200 [{ "id", "user": UserSummary, "created_at" }]` (creator only) |
| POST | `/api/group-requests/{requestId}/accept` | none | `204` (creator only) |
| POST | `/api/group-requests/{requestId}/decline` | none | `204` (creator only) |
| GET | `/api/groups/{id}/posts?before=` | none | `200 [Post]` (members only, otherwise `403`) |
| POST | `/api/groups/{id}/posts` | multipart: `content`, optional `image` | `201 Post` (members only) |
| GET | `/api/groups/{id}/events` | none | `200 [Event]` (members only) |
| POST | `/api/groups/{id}/events` | `{"title","description","event_time":"2026-11-01T18:00"}` | `201 Event`; every other member gets a notification |
| POST | `/api/events/{id}/respond` | `{"response":"going"}` or `"not_going"` | `200 Event` (members only; can be changed) |

```jsonc
// Group
{ "id", "title", "description", "creator": UserSummary, "member_count",
  "my_status": "none" | "member" | "invited" | "requested", "invite_id": null, "created_at" }

// Event
{ "id", "group_id", "title", "description", "event_time", "creator": UserSummary,
  "going_count", "not_going_count", "my_response": null | "going" | "not_going" }
```

Comments on group posts use the normal `/api/posts/{id}/comments` endpoints. Access is checked by group membership.

## Notifications (Phase 2 to 4)

Every query is limited to the logged-in user's own rows: someone else's notification id behaves exactly like an unknown one (`404`).

| Method | Path | Response |
|---|---|---|
| GET | `/api/notifications?before={id}` | `200 [Notification]`: newest first, 50 per page. Pass the last `id` of a page as `before` for the next page. `400` if `before` is not a number |
| GET | `/api/notifications/unread-count` | `200 {"unread": 3}` |
| POST | `/api/notifications/{id}/read` | `204`. `404` if it is not yours or does not exist |
| POST | `/api/notifications/read-all` | `204` |

```jsonc
// Notification
{ "id": 12, "type": "follow_request", "actor": UserSummary | null, "ref_id": null,
  "is_read": false, "created_at": "2026-10-13 21:35:45" }
```

`actor` is the user who caused the notification. It is `null` only if that account no longer exists. Phase 3 will add `group` and `event` objects for the group types.

| type | Sent to | actor | ref_id | Lifetime |
|---|---|---|---|---|
| `follow_request` | private user who got a request | requester | null | **Removed** when the request is accepted, declined or cancelled, or when the user goes public. Answer it with `POST /api/follow-requests/{actor.id}/accept` or `/decline` |
| `new_follower` (extra) | public user who got a follower | follower | null | stays |
| `follow_accepted` (extra) | requester whose request was accepted (by hand, or by the user going public) | the user who accepted | null | stays |
| `group_invite` (Phase 3) | invitee | inviter | group id | |
| `group_join_request` (Phase 3) | group creator | requester | group id | |
| `group_event` (Phase 3) | every group member except the creator | event creator | event id | |

### Live updates

Over the WebSocket (`/ws`), the recipient's open tabs get:

| type | payload | when |
|---|---|---|
| `notification` | `Notification` | a notification was created for you |
| `notification_removed` | `{"id": 12}` | one of your notifications was resolved and deleted (e.g. a follow request was cancelled) |

Pushes are sent only after the database transaction has committed. A user with no open tab gets nothing pushed; `GET /api/notifications` shows everything on the next page load.

## Chat (Phase 4)

| Method | Path | Response |
|---|---|---|
| GET | `/api/chats` | `200 { "users": [UserSummary + "unread_count"], "groups": [{ "id", "title" }] }`: people you can message (follow either way) and your groups |
| GET | `/api/messages/{userId}?before={messageId}` | `200 [Message]`: 50 per page, newest last; `403` if neither of you follows the other. Marks them read |
| GET | `/api/groups/{id}/messages?before=` | `200 [GroupMessage]`: members only |

## WebSocket `/ws` (hub since Phase 2 step 0; message types added in Phases 2 to 4)

The connection is authenticated by the session cookie during the upgrade (`401` if invalid). The upgrade is also refused with `403` when the browser's `Origin` header is not `FRONTEND_ORIGIN`; requests without an `Origin` header (non-browser clients) are allowed, the cookie is still required. The Next.js proxy forwards both the cookie and the `Origin` header, so the page connects to `ws://<frontend host>/ws`.

Each tab has its own connection; an event for a user goes to all of that user's tabs. The server sends a ping every 30 s and drops a connection that sends nothing (not even a pong) for 60 s. Browsers answer pings automatically. Events for a user with no open connection are not queued: the data is in the database and the page loads it.

Right now the server only sends events (`notification` and `notification_removed`, see Notifications) and ignores frames from the browser. Every frame is a JSON envelope:

```jsonc
{ "type": "...", "payload": { ... } }
```

| Direction | type | payload |
|---|---|---|
| client → server | `private_message` | `{ "receiver_id": 7, "content": "hi 😀" }` |
| client → server | `group_message` | `{ "group_id": 2, "content": "hey all" }` |
| server → client | `private_message` | `Message`: sent to the receiver's tabs and echoed to the sender's tabs |
| server → client | `group_message` | `GroupMessage`: sent to every online member |
| server → client | `notification` | `Notification` |
| server → client | `notification_removed` | `{ "id": 12 }` |
| server → client | `error` | `{ "message": "you can't message this user" }` |

```jsonc
// Message
{ "id", "sender": UserSummary, "receiver_id", "content", "created_at" }
// GroupMessage
{ "id", "group_id", "sender": UserSummary, "content", "created_at" }
```

Rules checked by the server on every send:
- Private message: at least one of the two users follows the other. The message is always stored. It is pushed live to the receiver when the receiver follows the sender or has a public profile.
- Group message: the sender must be a member.
- Content is 1–2000 characters of UTF-8. Emojis are stored as-is.
