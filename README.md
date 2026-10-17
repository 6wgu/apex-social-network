# Social Network

A full-stack social network with profiles, follow requests, privacy-aware posts, comments, groups, events, notifications, and real-time private and group chat.

- **Backend:** Go (`net/http`), SQLite (`mattn/go-sqlite3`), `golang-migrate`, `gorilla/websocket`, `bcrypt`, `uuid`
- **Frontend:** Next.js (App Router)
- **Docs:** [API contract](docs/api.md), [ERD](docs/erd.md), [Phase 5 report](docs/phase5-progress-report.md), [Arabic demo dataset report](docs/arabic-demo-seed-report.md)

## Features

- Session-based authentication with HTTP-only cookies and bcrypt password hashes
- Public and private profiles with follow requests and access-controlled content
- Posts and comments with JPEG, PNG, GIF, and WebP uploads and three privacy levels
- Groups, invitations, join requests, events, RSVPs, and group-only content
- Private chat and group chat over authenticated WebSockets, with emoji support
- Live notifications with separate notification and unread-message badges

## Quick start with Docker

```sh
docker compose up --build
```

Open the frontend at http://localhost:3000. The backend health endpoint is http://localhost:8080/api/health.

| Image | Container port | URL |
|---|---|---|
| `social-network-frontend` | 3000 | http://localhost:3000 (open this one) |
| `social-network-backend` | 8080 | http://localhost:8080/api/health |

The SQLite database and uploaded images are stored in the named volumes `db-data` and `uploads`, so they survive container restarts. To reset all local data and apply every migration from scratch, run:

```sh
docker compose down -v && docker compose up --build
```

Stop the stack without deleting data with:

```sh
docker compose down
```

## Local test account

This account exists in the current development database only. Do not use it in production:

| Login email | Password |
|---|---|
| `demo@social-network.local` | `DemoPass123!` |

The application uses email as the login identifier. Reset the database with `docker compose down -v` and this account will be removed.

## Verification

Run the backend checks from `backend/`:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Run the frontend production build from `frontend/`:

```sh
npm ci
npm run build
```

The current results are recorded in the [Phase 5 report](docs/phase5-progress-report.md). The 17 October 2026 verification passed the backend tests, race detector, vet, frontend production build, Docker image build, container startup, health checks, and fresh-volume migration startup check. Browser smoke checks also confirmed the login and registration routes, unauthenticated protection for private routes, a responsive 390px registration layout, and rejection of non-image avatar selections.

The WebSocket server uses a two-minute idle timeout. The frontend production dependency audit currently reports one moderate and one high advisory through Next.js/PostCSS; resolving them requires the breaking `npm audit fix --force` upgrade to Next.js 16 and was intentionally left for a dependency upgrade review.

## Run locally without Docker

Requires Go 1.26+ with a C compiler (CGO is needed for SQLite) and Node.js 20+.

```sh
# Terminal 1
cd backend
go run .

# Terminal 2
cd frontend
npm install
npm run dev
```

The local frontend runs at http://localhost:3000 and proxies API, upload, and WebSocket traffic to the backend at http://localhost:8080.

Backend environment variables (all optional):

| Variable | Default |
|---|---|
| `PORT` | `8080` |
| `DB_PATH` | `data/social-network.db` |
| `UPLOAD_DIR` | `uploads` |
| `MIGRATIONS_PATH` | `file://pkg/db/migrations/sqlite` |
| `FRONTEND_ORIGIN` | `http://localhost:3000,http://127.0.0.1:3000`: comma-separated list of the browser origins allowed to open `/ws` (exact matches, no wildcard), also used for CORS when the frontend calls `:8080` directly |
| `COOKIE_SECURE` | unset. Set to `1` when the site is served over HTTPS, to add the `Secure` flag to the session cookie |

## Migrations and database reset

Migrations live in `backend/pkg/db/migrations/sqlite` as `NNNNNN_name.up.sql` / `.down.sql`. All pending ones are applied on every start.

```sh
cd backend
go run . -migrate=down
go run .
```

Every new table or column requires a new numbered migration. Never edit a migration that has already been merged. Pending migrations are applied automatically when the backend starts.

Rolling back `000014_add_group_id_to_posts` deletes every group post (the `group_id` column they depend on is dropped; their comments go with them). Their image files are **not** deleted: they stay in `uploads/` with nothing pointing to them. `/uploads/` treats a file that belongs to no post or comment like an avatar, so any logged-in user who knows its URL can still fetch it. Delete those files by hand after such a rollback.

## Documentation

- [API contract](docs/api.md): routes, request bodies, responses, and WebSocket envelopes
- [ERD](docs/erd.md): database tables and relationships
- [Phase 5 report](docs/phase5-progress-report.md): verification evidence, decisions, and remaining optional coverage

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
