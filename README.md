# APEX — Connect at Full Speed.

A full-stack social network with a Formula-racing look: profiles, follow requests, privacy-aware posts with likes and comments, groups, events, live notifications, and real-time private and group chat.

- **Backend:** Go (`net/http`), SQLite (`mattn/go-sqlite3`), `golang-migrate`, `gorilla/websocket`, `bcrypt`, `uuid`
- **Frontend:** Next.js 15 (App Router, plain JavaScript and CSS, no UI library)
- **Docs:** [API contract](docs/api.md), [ERD](docs/erd.md), [Phase 5 report](docs/phase5-progress-report.md), [Arabic demo dataset report](docs/arabic-demo-seed-report.md)

## Features

**Accounts and privacy**
- Session-based authentication with HTTP-only cookies and bcrypt password hashes
- Registration checks the email format, a strong password (8+ characters, upper and lower case, a number and a special character, ticked off live under the field) and an age of 7 to 120, in the browser and again on the server, with each error shown next to its field
- Email verification is required: until a user enters the 6-digit code emailed to them, every page sends them to `/verify` and the API answers `403` to everything except `/api/me`, the code routes and logout
- Public and private profiles (a switch on the profile and in Settings), follow requests, and access-controlled content
- Settings page: nickname, privacy, about text, avatar, cursor preference and log out

**Posts**
- Posts and comments with JPEG, PNG, GIF and WebP images up to 50 MB, and three privacy levels (public, followers, chosen followers)
- Likes on posts
- New posts, comments and like counts appear live on every open page that may show them, with no reload
- People search

**Groups and chat**
- Groups, invitations, join requests, events with RSVPs, and group-only posts
- Private chat and group chat over authenticated WebSockets, with emoji, day separators and unread badges

**Notifications**
- Live notifications with a bell, an unread-message badge and pop-up toasts
- Pages also re-check every 20–30 seconds and whenever the tab gets focus, so nothing is missed after a dropped connection or a sleeping laptop

**Design**
- Carbon-black theme with racing red and cyan, Space Grotesk and DM Sans fonts (self-hosted)
- Landing page with a floating Formula car, starting-light loaders, a launch animation on log in and a farewell animation on log out
- Optional Formula-car mouse cursor (Settings), collapsible sidebar, responsive down to phone width, and reduced-motion support
- Branded HTML emails (welcome and verification code) with the APEX logo

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

## Starting accounts

The repository ships a starting database, [`backend/pkg/db/initial/social-network.db`](backend/pkg/db/initial), with the images it uses. When the backend starts and finds no database yet (a fresh clone, or after `docker compose down -v`), it copies that one in; an existing database is never touched. It holds these two accounts, abs's first post ("Im the webmaster") with its replies and likes, and their chat. Do not use these passwords in production:

| User | Login email | Password |
|---|---|---|
| abs (user 1, the webmaster) | `abbasazhar20200@gmail.com` | `P@ssw0rd` |
| Developer (user 2) | `demo@social-network.local` | `DemoPass123!` |

The application uses email as the login identifier. `docker compose down -v` deletes all data; on the next start the backend installs the starting database again.

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

## Email and verification

The backend sends email through Gmail SMTP with an App Password. Until credentials are set it runs in **console mode**: every email (and every verification code) is printed in the backend log instead of being sent, so the app and its tests work without them.

1. Use a Gmail address for the app and turn on 2-Step Verification for it.
2. Create an App Password at https://myaccount.google.com/apppasswords (16 letters).
3. Copy the settings file to the project root and fill in only the two credential lines:

   ```sh
   cp backend/.env.example .env
   ```

   Then set `SMTP_USER=yourapp@gmail.com` and `SMTP_PASS=abcd efgh ijkl mnop` in `.env`, and restart the backend (`docker compose up -d --build backend`).

4. Check the login and send yourself a test email:

   ```sh
   go run ./cmd/test-email
   go run ./cmd/test-email -otp
   ```

`.env` is git-ignored and kept out of Docker images; never commit the App Password. Docker Compose reads `.env` in the project root and `backend/.env` when they exist; `go run .` finds `.env` in `backend/` or any parent folder. The backend log says `email: Gmail login OK` when the credentials work. If port 587 is blocked on your network, use `SMTP_PORT=465` and `SMTP_SECURE=true`.

| Variable | Default | Meaning |
|---|---|---|
| `SMTP_USER`, `SMTP_PASS` | empty | Gmail address and its App Password; empty means console mode |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_SECURE` | `smtp.gmail.com`, `587`, `false` | Mail server |
| `MAIL_FROM_NAME` | `APEX` | Sender name |
| `OTP_LENGTH`, `OTP_TTL_MINUTES`, `OTP_MAX_ATTEMPTS`, `OTP_RESEND_SECONDS` | `6`, `10`, `5`, `60` | Verification code rules |

After registering, users land on `/verify`, where a code is emailed to them. Verification is required: until the code is entered, the rest of the site is locked (pages redirect to `/verify`, the API answers `403 verify your email first`). In console mode, read the code with `docker compose logs backend`.

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
    ├── mail/                  Gmail SMTP mailer (console mode without credentials) and email templates
    ├── otp/                   one-time verification codes: hashed, expiring, limited tries, resend cooldown
    ├── handlers/              routes and one file per feature (auth, otp, verified, profiles, follows, posts,
    │                          likes, comments, live, notifications, groups, events, messages,
    │                          group_messages, upload)
    ├── middleware/            logging, panic recovery, CORS, RequireAuth
    ├── models/                structs shared by DB and JSON
    ├── session/               sessions table + session_id cookie
    └── ws/                    WebSocket hub: live connections per user, push JSON events, hand incoming
                               frames (chat) to a handler
frontend/
├── app/                       Next.js pages: landing/feed, login, register, verify, profile, search,
│                              groups, chats, settings; apex.css holds the whole theme
├── components/                Navbar (sidebar), AuthProvider (current user, route guard, verification gate,
│                              logout animation), RealtimeProvider (one WebSocket per tab), NotificationBell,
│                              Toaster (notify() pop-ups), Landing, LaunchOverlay, RacingLights, RacingCursor,
│                              PostComposer, NewPostButton, PostCard (likes + live comments), PostList,
│                              FollowButton, FollowRequests, UserList, Avatar, Field, Icon, ImagePicker,
│                              GroupJoinButton, GroupEvents, InviteDialog, JoinRequests, Modal,
│                              ChatWindow (private and group chat), EmojiPicker, MessageButton,
│                              UnreadMessages, Footer
├── lib/api.js                 fetch wrapper (credentials, JSON errors, 401 → /login)
├── lib/usePagedPosts.js       newest-first post lists with "Load more" and live refresh of the top
├── lib/useAutoRefresh.js      re-checks data on a timer and when the tab gets focus
├── public/fonts/              Space Grotesk and DM Sans
└── next.config.mjs            proxies /api, /uploads, /ws to the backend (2-minute idle timeout)
```
