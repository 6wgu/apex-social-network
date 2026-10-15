# Social Network

A Facebook-like social network: followers, profiles, posts, groups, notifications and chat.

- **Backend:** Go (`net/http`), SQLite (`mattn/go-sqlite3`), `golang-migrate`, `gorilla/websocket`, `bcrypt`, `uuid`
- **Frontend:** Next.js (App Router)
- **Docs:** [API contract](docs/api.md), [ERD](docs/erd.md)

## Run with Docker

```sh
docker compose up --build
```

| Image | Container port | URL |
|---|---|---|
| `social-network-frontend` | 3000 | http://localhost:3000 (open this one) |
| `social-network-backend` | 8080 | http://localhost:8080/api/health |

The SQLite file and uploaded images are stored in the named volumes `db-data` and `uploads`, so they survive restarts. To start from an empty database, run:

```sh
docker compose down -v && docker compose up --build
```

## Run locally (without Docker)

Requires Go 1.26+ with a C compiler (CGO is needed for SQLite) and Node.js 20+.

```sh
# terminal 1
cd backend
go run .                  # applies migrations, then listens on :8080

# terminal 2
cd frontend
npm install
npm run dev               # http://localhost:3000
```

Backend environment variables (all optional):

| Variable | Default |
|---|---|
| `PORT` | `8080` |
| `DB_PATH` | `data/social-network.db` |
| `UPLOAD_DIR` | `uploads` |
| `MIGRATIONS_PATH` | `file://pkg/db/migrations/sqlite` |
| `FRONTEND_ORIGIN` | `http://localhost:3000,http://127.0.0.1:3000`: comma-separated list of the browser origins allowed to open `/ws` (exact matches, no wildcard), also used for CORS when the frontend calls `:8080` directly |
| `COOKIE_SECURE` | unset. Set to `1` when the site is served over HTTPS, to add the `Secure` flag to the session cookie |

## Migrations

Migrations live in `backend/pkg/db/migrations/sqlite` as `NNNNNN_name.up.sql` / `.down.sql`. All pending ones are applied on every start.

```sh
cd backend
go run . -migrate=down    # roll back every migration and exit
go run .                  # apply them again and start
go test ./...             # migrations (up/down, one by one), sessions, auth, profile, post and group rules, WebSocket hub
```

Rule: a new table or column means a new numbered migration. Never edit a migration that has already been merged.

Rolling back `000014_add_group_id_to_posts` deletes every group post (the `group_id` column they depend on is dropped; their comments go with them). Their image files are **not** deleted: they stay in `uploads/` with nothing pointing to them. `/uploads/` treats a file that belongs to no post or comment like an avatar, so any logged-in user who knows its URL can still fetch it. Delete those files by hand after such a rollback.

## Project structure

```
backend/
├── server.go                  entry point: config, DB, migrations, middleware, server
└── pkg/
    ├── db/
    │   ├── migrations/sqlite/ numbered .up.sql / .down.sql files
    │   └── sqlite/sqlite.go   connection (FKs, WAL, busy timeout) + migrate up/down
    ├── handlers/              routes and one file per feature (auth, profiles, follows, posts, comments,
    │                          notifications, groups, events, messages, group_messages, upload)
    ├── middleware/            logging, panic recovery, CORS, RequireAuth
    ├── models/                structs shared by DB and JSON
    ├── session/               sessions table + session_id cookie
    └── ws/                    WebSocket hub: live connections per user, push JSON events, hand incoming
                               frames (chat) to a handler
frontend/
├── app/                       Next.js pages (App Router)
├── components/                Navbar, AuthProvider (current user + route guard), RealtimeProvider (one WebSocket per tab),
│                              NotificationBell, FollowButton, EditProfile, UserList, Avatar,
│                              PostComposer, PostCard (with comments), PostList, ImagePicker,
│                              GroupJoinButton, GroupEvents, InviteDialog, JoinRequests, Modal,
│                              ChatWindow (private and group chat), EmojiPicker, MessageButton, UnreadMessages
├── lib/api.js                 fetch wrapper (credentials, JSON errors, 401 → /login)
├── lib/usePagedPosts.js       newest-first post lists with "Load more" (feed, profile and group posts)
└── next.config.mjs            proxies /api, /uploads, /ws to the backend
```
