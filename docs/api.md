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
  "privacy": "public",            // public | almost_private | private (always "public" for a group post, and ignored)
  "group_id": null,               // the group of a group post, null for a profile post
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
| POST | `/api/logout` | none | `204`, always. Public route: deletes the session row if the cookie names one, and always clears the cookie, even when it is expired or unknown. For a valid session it also closes every open `/ws` connection of that user |
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
| GET | `/api/users/{id}/posts?before={postId}` | none | `200 [Post]`: that user's profile posts that you may see, newest first, 20 per page (group posts are not listed). `403 "this profile is private"` for a private profile you neither own nor follow, `404` unknown user, `400` bad `before` |
| GET | `/api/feed?before={postId}` | none | `200 [Post]`: every profile post you may see, your own included, newest first, 20 per page. Group posts stay in their group. The privacy rule is part of the SQL query, so hidden posts are never loaded. `400` bad `before` |
| GET | `/api/posts/{id}/comments?after={commentId}` | none | `200 [Comment]`: oldest first, 50 per page (pass the last `id` as `after`). `404` if the post does not exist **or you may not see it**. `400` bad `after` |
| POST | `/api/posts/{id}/comments` | multipart: `content` (up to 2000 characters), optional `image` (JPEG/PNG/GIF, 5 MB) | `201 Comment`. `404` if you may not see the post (visibility is checked again in the `INSERT` itself). `400` if both are empty, the text is too long, or the image is invalid |

Who can see a post (`CanViewPost`). The rule is a single SQL condition, `postVisibleSQL` in `pkg/handlers/posts.go`, and every endpoint that returns posts uses it:
- A **group post** (`group_id` set) is visible to the group's members only, whatever its `privacy` says. None of the rules below apply to it. Its comments and images follow it, so they are members-only too.
- The author always sees their own (profile) post.
- `public`: every logged-in user when the author's profile is public. When the profile is **private**, only the author's accepted followers can see it: a private profile shows its posts to followers only, whatever each post's own setting.
- `almost_private`: the author's accepted followers.
- `private`: the followers chosen in `post_viewers`, **while they still follow** the author. Unfollowing takes the access away; following again gives it back.

`GET /api/users/{id}/posts` also applies the profile rule (`CanViewProfile`) first, answering `403` instead of an empty list for a private profile you neither own nor follow.

**Comments** are visible to exactly the users who can see their post. Losing access to a post (for example by unfollowing) also takes away its comments and their images.

**Images.** `GET /uploads/{name}` checks whether the file belongs to a post or to a comment on a post. If it does, it applies that post's `CanViewPost`, and someone who may not see the post gets the same `404` as for a missing file. Any other stored image (an avatar) is served to every logged-in user. Post images are sent with `Cache-Control: private, no-cache`.

## Groups and events (Phase 3)

Every logged-in user can browse groups and see a group's title, description, creator and member count. Everything inside a group (members, posts, comments, images, events) is for members only, checked in the backend on every endpoint. There are two ways in: an **invitation** from any member, which only the invitee can answer, or a **join request**, which only the group's creator can answer. Invitations and requests exist only while pending; answering deletes them. Joining either way also deletes the other one. Members are never removed (there is no leave or kick).

| Method | Path | Body | Response |
|---|---|---|---|
| POST | `/api/groups` | `{"title","description"}` | `201 Group`; the creator becomes the first member. `400` if either is empty, title over 100 or description over 2000 characters |
| GET | `/api/groups` | none | `200 [Group]`: every group, newest first, each with your `my_status` |
| GET | `/api/groups/{id}` | none | `200 Group`. Members also get `members` and `invited_users`. `404` unknown group |
| POST | `/api/groups/{id}/invite` | `{"user_id": 7}` | `201 {"id","group_id","user_id"}`; the invitee gets `group_invite`. `403` if you are not a member, `404` unknown user, `400` no `user_id`, `409 "this user is already a member of the group"` or `409 "this user is already invited to the group"` |
| POST | `/api/group-invites/{inviteId}/accept` | none | `204`: you are a member (your own pending join request to that group is removed too). `404` if the invitation does not exist **or is not addressed to you** |
| POST | `/api/group-invites/{inviteId}/decline` | none | `204`: the invitation is deleted; a member may invite you again. `404` as for accept |
| POST | `/api/groups/{id}/request` | none | `201 {"id","group_id"}`; the creator gets `group_join_request`. `409` if you are already a member, already invited (accept the invitation instead), or already asked |
| DELETE | `/api/groups/{id}/request` | none | `204`: cancels your pending request (it leaves the creator's bell). `404` if you have none |
| GET | `/api/groups/{id}/requests` | none | `200 [{ "id", "user": UserSummary, "created_at" }]`, oldest first. `403` for anyone but the creator |
| POST | `/api/group-requests/{requestId}/accept` | none | `204`: the requester is a member and gets `group_request_accepted`. `403` for anyone but the group's creator, `404` unknown or already answered |
| POST | `/api/group-requests/{requestId}/decline` | none | `204`: the request is deleted; the user may ask again. `403` / `404` as for accept |
| GET | `/api/groups/{id}/posts?before={postId}` | none | `200 [Post]`, newest first, 20 per page. `403` for non-members |
| POST | `/api/groups/{id}/posts` | multipart: `content` (up to 5000 characters), optional `image` | `201 Post` with `group_id` set. No privacy field: the group decides. `403` for non-members, `400` as for `/api/posts` |
| GET | `/api/groups/{id}/events` | none | `200 [Event]`, by date, soonest first. `403` for non-members |
| POST | `/api/groups/{id}/events` | `{"title","description","event_time"}` | `201 Event`; every other member gets `group_event`. `event_time` is RFC 3339 (`"2026-11-01T17:00:00Z"`, or with an offset); without a zone (`"2026-11-01T17:00"`) it is taken as UTC. `400` if a field is empty or too long, the time is not a date, or it is not in the future. `403` for non-members |
| POST | `/api/events/{id}/respond` | `{"response":"going"}` or `"not_going"` | `200 Event` with fresh counts. Answering again changes your answer, until the event starts. `400` any other value, `400 "this event has already happened"` once its time has passed (earlier answers are kept), `404` if the event does not exist **or is in a group you are not in** |

```jsonc
// Group
{ "id", "title", "description", "creator": UserSummary, "member_count",
  "my_status": "none" | "member" | "invited" | "requested",
  "invite_id": null,          // your pending invitation's id when my_status is "invited" (answer it with /api/group-invites/{id}/...)
  "created_at",
  "members": [UserSummary],          // members only, in joining order
  "invited_users": [UserSummary] }   // members only: pending invitations

// Event
{ "id", "group_id", "title", "description", "event_time": "2026-11-01 17:00:00", // UTC
  "creator": UserSummary, "going_count", "not_going_count",
  "my_response": null | "going" | "not_going", "created_at" }
```

Comments on group posts use the normal `/api/posts/{id}/comments` endpoints, and group post images the normal `/uploads/` path: both follow the post, so a non-member gets `404`, exactly as for a missing post.

`IsGroupMember(user, group)` in `pkg/handlers/groups.go` is the membership check; group chat (Phase 4) uses it too.

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
{ "id": 12, "type": "follow_request", "actor": UserSummary, "ref_id": null,
  "group": null,   // { "id", "title" } for the group types
  "event": null,   // { "id", "title", "event_time" } for group_event
  "is_read": false, "created_at": "2026-10-13 21:35:45" }
```

`actor` is the user who caused the notification. It is always set: when a user is deleted, the notifications they caused are deleted with them (`actor_id` is `ON DELETE CASCADE`). For the group types the API adds the `group` (and for `group_event` the `event`) that `ref_id` points at, so the bell can name them without another request.

| type | Sent to | actor | ref_id | Lifetime |
|---|---|---|---|---|
| `follow_request` | private user who got a request | requester | null | **Removed** when the request is accepted, declined or cancelled, or when the user goes public. Answer it with `POST /api/follow-requests/{actor.id}/accept` or `/decline` |
| `new_follower` (extra) | public user who got a follower | follower | null | stays |
| `follow_accepted` (extra) | requester whose request was accepted (by hand, or by the user going public) | the user who accepted | null | stays |
| `group_invite` | invitee | the member who invited | group id | **Removed** when the invitation is accepted or declined. Answer it with `GET /api/groups/{group.id}` (its `invite_id`), then `POST /api/group-invites/{invite_id}/accept` or `/decline` |
| `group_join_request` | the group's creator only | requester | group id | **Removed** when the creator accepts or declines, the requester cancels, or the requester joins through an invitation. Answer it from `GET /api/groups/{group.id}/requests` |
| `group_request_accepted` (extra) | requester whose join request was accepted | the creator | group id | stays |
| `group_event` | every member of the group except the event's creator | event creator | event id | stays |

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

## WebSocket `/ws` (Phases 2 to 4)

The connection is authenticated by the session cookie during the upgrade (`401` if invalid). The upgrade is also refused with `403` when the browser's `Origin` header is not one of the origins listed in `FRONTEND_ORIGIN` (comma-separated, exact matches; the default allows `http://localhost:3000` and `http://127.0.0.1:3000`); requests without an `Origin` header (non-browser clients) are allowed, the cookie is still required. The Next.js proxy forwards both the cookie and the `Origin` header, so the page connects to `ws://<frontend host>/ws`.

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
