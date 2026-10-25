# APEX · social-network

**Connect at full speed.** A full-stack social network with profiles, followers, posts, groups, events and real-time chat.

**<a href="https://apex.zerot.dev/" target="_blank" rel="noopener noreferrer">▶ Live demo</a>**: try it in the browser with no setup. Sign in with any [demo account](#demo-accounts).

<a href="docs/screenshots/01-landing.webp"><img src="docs/screenshots/01-landing.webp" alt="APEX landing page"></a>

## Contents

- <a href="https://apex.zerot.dev/" target="_blank" rel="noopener noreferrer">Live demo</a>
- [About](#about)
- [Features](#features)
- [Screenshots](#screenshots)
- [Tech stack](#tech-stack)
- [Getting started](#getting-started)
- [Demo accounts](#demo-accounts)
- [Project structure](#project-structure)
- [Database](#database)
- [API](#api)
- [Security and privacy](#security-and-privacy)
- [Testing](#testing)
- [Team](#team)

## About

APEX is our Reboot01 **social-network** project: a Facebook-like social network.
People sign up, choose who can see their profile and posts, follow each other, join groups, plan events and chat in real time.
The Go backend enforces every rule, so privacy does not depend on the browser.
The Next.js frontend gives it a dark, Formula-racing look that works from desktop down to phone width.

## Features

### Accounts & security

- Sign up with email, password, first and last name and date of birth; nickname, about me and avatar are optional.
- A live checklist under the password field: 8+ characters, upper and lower case, a number and a special character.
- Every rule is checked in the browser and again on the server, with the error shown next to its field.
- Email verification with a 6-digit code. Until it is entered, only the verify page works. A mistyped email can be changed there.
- Accounts left unverified for 24 hours are deleted, so their email can be used again.
- "Forgot password?": a code by email, then a new password. The answers never reveal whether an account exists, and a reset signs out every device.

### Profiles & followers

- Public or private profile, switched on the profile page or in Settings.
- Following a public profile is instant. A private profile gets a follow request to accept or decline.
- A profile shows the user's details, posts, followers and following. A private profile shows only the name and avatar to non-followers.
- People search by name or nickname.
- Settings for nickname (unique), about me, avatar, privacy and the optional racing-car mouse cursor.

### Posts & comments

- Posts and comments with text, an image or a GIF (JPEG, PNG, GIF or WebP, up to 50 MB).
- Three privacy levels per post: **public**, **almost private** (followers only) and **private** (only the followers you pick).
- Likes, and deleting your own posts and comments (a post's author can also remove comments on it).
- New posts, comments and like counts appear live, without a reload.

### Groups & events

- Create groups, browse all groups, and filter them by title or description.
- Join by invitation from any member, or by a join request that the creator accepts.
- Group posts, comments, members and events are visible to members only.
- Events with a title, description and date. Members answer **Going** or **Not going**.

### Chat

- Private chat between two users when at least one follows the other.
- One chat room per group, for members only.
- Real-time messages over WebSockets, with an emoji picker, a typing indicator, day separators and unread badges.
- Messages are stored, so offline users see them on their next visit.

### Notifications

- A bell with live notifications: follow requests, group invitations, join requests, new events, new followers and accepted requests.
- Follow requests, invitations and join requests can be answered right from the bell.
- Unread badges for notifications and for messages, pop-up toasts, and "Mark all as read".

## Screenshots

<table>
  <tr>
    <td><a href="docs/screenshots/02-register.webp"><img src="docs/screenshots/02-register.webp" alt="Register page"></a><br><b>Register</b>: the password checklist ticks off as you type</td>
    <td><a href="docs/screenshots/03-login.webp"><img src="docs/screenshots/03-login.webp" alt="Login page"></a><br><b>Log in</b> with email and password</td>
  </tr>
  <tr>
    <td><a href="docs/screenshots/04-verify.webp"><img src="docs/screenshots/04-verify.webp" alt="Verify email page"></a><br><b>Verify email</b> with a 6-digit code</td>
    <td><a href="docs/screenshots/05-forgot-password.webp"><img src="docs/screenshots/05-forgot-password.webp" alt="Forgot password page"></a><br><b>Forgot password</b>: a reset code by email</td>
  </tr>
  <tr>
    <td><a href="docs/screenshots/06-feed.webp"><img src="docs/screenshots/06-feed.webp" alt="Feed"></a><br><b>Feed</b> with friends and suggested groups</td>
    <td><a href="docs/screenshots/07-create-post.webp"><img src="docs/screenshots/07-create-post.webp" alt="Create post dialog"></a><br><b>New post</b> with an image and the privacy picker</td>
  </tr>
  <tr>
    <td><a href="docs/screenshots/08-profile.webp"><img src="docs/screenshots/08-profile.webp" alt="Own profile"></a><br><b>Own profile</b> with the privacy switch</td>
    <td><a href="docs/screenshots/09-private-profile.webp"><img src="docs/screenshots/09-private-profile.webp" alt="Private profile seen by a stranger"></a><br><b>Private profile</b> seen by someone who does not follow</td>
  </tr>
  <tr>
    <td><a href="docs/screenshots/10-followers.webp"><img src="docs/screenshots/10-followers.webp" alt="Followers tab"></a><br><b>Followers</b>, with a pending follow request</td>
    <td><a href="docs/screenshots/11-notifications.webp"><img src="docs/screenshots/11-notifications.webp" alt="Notification bell open"></a><br><b>Notifications</b>, answered from the bell</td>
  </tr>
  <tr>
    <td><a href="docs/screenshots/12-groups.webp"><img src="docs/screenshots/12-groups.webp" alt="Groups browser"></a><br><b>Groups</b>: invitations, your groups and more to discover</td>
    <td><a href="docs/screenshots/13-group.webp"><img src="docs/screenshots/13-group.webp" alt="Group page"></a><br><b>Group page</b> with members-only posts</td>
  </tr>
  <tr>
    <td><a href="docs/screenshots/14-group-events.webp"><img src="docs/screenshots/14-group-events.webp" alt="Group events"></a><br><b>Events</b> with Going / Not going</td>
    <td><a href="docs/screenshots/15-chat.webp"><img src="docs/screenshots/15-chat.webp" alt="Private chat"></a><br><b>Private chat</b> with emoji and unread badges</td>
  </tr>
  <tr>
    <td><a href="docs/screenshots/16-group-chat.webp"><img src="docs/screenshots/16-group-chat.webp" alt="Group chat"></a><br><b>Group chat</b> for the members</td>
    <td><a href="docs/screenshots/17-settings.webp"><img src="docs/screenshots/17-settings.webp" alt="Settings page"></a><br><b>Settings</b>: nickname, privacy, about me and avatar</td>
  </tr>
</table>

### On a phone

<table>
  <tr>
    <td><a href="docs/screenshots/18-phone-feed.webp"><img src="docs/screenshots/18-phone-feed.webp" alt="Feed on a phone" width="240"></a><br><b>Feed</b></td>
    <td><a href="docs/screenshots/19-phone-chat.webp"><img src="docs/screenshots/19-phone-chat.webp" alt="Chat on a phone" width="240"></a><br><b>Chat</b></td>
    <td><a href="docs/screenshots/20-phone-profile.webp"><img src="docs/screenshots/20-phone-profile.webp" alt="Profile on a phone" width="240"></a><br><b>Profile</b></td>
  </tr>
</table>

The screenshots use made-up demo users and pictures.

## Tech stack

| Part | Technology |
|---|---|
| Backend | Go (`net/http` standard library router), SQLite (`mattn/go-sqlite3`), `golang-migrate`, `gorilla/websocket`, `bcrypt` (`golang.org/x/crypto`), `google/uuid` |
| Frontend | Next.js 15 (App Router) and React 19, plain JavaScript and CSS, self-hosted fonts |
| Containers | Docker multi-stage images for the backend and the frontend, Docker Compose |

Only the packages allowed by the project are used. The backend has no web framework and no ORM. The frontend has no UI library, and its unit tests use Node's built-in test runner.

## Getting started

> Just want to look around? Open the **[live demo](https://your-demo-url.com)**. The steps below are only for running APEX yourself.

### With Docker

From the project root:

```bash
docker compose up --build
```

| Service | URL |
|---|---|
| Frontend (open this one) | http://localhost:3000 |
| Backend health check | http://localhost:8080/api/health |

The database and uploaded images live in the named volumes `db-data` and `uploads`, so they survive restarts.

```bash
docker compose logs -f backend   # backend log (email codes in console mode)
docker compose down              # stop, keep the data
docker compose down -v           # stop and delete all data
```

### Without Docker

Requirements:

- **Go 1.26+** with cgo, which needs a C compiler (`gcc`) for SQLite.
- **Node.js 22** (20.11 or newer works) and npm.
- **Windows:** install a MinGW-w64 `gcc` (for example from MSYS2 or TDM-GCC), add it to `PATH`, and check that `go env CGO_ENABLED` prints `1`. If a built `.exe` refuses to start, build with `go build -ldflags="-s -w"`, as the Dockerfile does.

Start the backend (http://localhost:8080):

```bash
cd backend
go run .
```

Start the frontend (http://localhost:3000) in a second terminal:

```bash
cd frontend
npm install
npm run dev
```

The frontend proxies `/api`, `/uploads` and `/ws` to the backend, so the browser only talks to port 3000. Set `BACKEND_URL` before `npm run dev` (or `npm run build`) if the backend runs somewhere else.

### Configuration

All settings are optional. Copy the example file and edit what you need:

```bash
cp backend/.env.example backend/.env
```

`go run .` reads `backend/.env` (or a `.env` in a parent folder), and Docker Compose reads `backend/.env` and `./.env`. Variables already set in the environment win.

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | Backend port |
| `DB_PATH` | `data/social-network.db` | SQLite database file |
| `UPLOAD_DIR` | `uploads` | Folder for uploaded images |
| `MIGRATIONS_PATH` | `file://pkg/db/migrations/sqlite` | Migration files |
| `FRONTEND_ORIGIN` | `http://localhost:3000,http://127.0.0.1:3000` | Browser origins allowed to open the WebSocket (comma-separated) |
| `COOKIE_SECURE` | unset | `1` adds the `Secure` flag to the session cookie (use it behind HTTPS) |
| `SEED_DEMO` | `1` | Add the [demo accounts](#demo-accounts) when the database has no users. `0` turns it off |
| `SMTP_USER`, `SMTP_PASS` | empty | Gmail address and its App Password. Empty means console mode |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_SECURE` | `smtp.gmail.com`, `587`, `false` | Mail server |
| `MAIL_FROM_NAME` | `APEX` | Sender name |
| `OTP_LENGTH`, `OTP_TTL_MINUTES`, `OTP_MAX_ATTEMPTS`, `OTP_RESEND_SECONDS` | `6`, `10`, `5`, `60` | Email code rules |

**Email.** The backend sends its emails (welcome, verification code, reset code, password changed) in one of two modes:

- **Console mode** (default, no setup): `SMTP_USER` and `SMTP_PASS` are empty, nothing is sent, and every email, with its code, is printed in the backend terminal. With Docker, read it with `docker compose logs backend`.
- **SMTP with Gmail:** turn on 2-Step Verification for a Gmail account, create an App Password at https://myaccount.google.com/apppasswords, and put the address and the 16-letter password in `SMTP_USER` and `SMTP_PASS`. Restart the backend (`docker compose up -d --force-recreate backend` with Docker). The log then says `email: Gmail login OK`. If port 587 is blocked, use `SMTP_PORT=465` and `SMTP_SECURE=true`. In both modes every verification and reset code is also printed in the backend terminal (`verification code for …` / `reset code for …`), and a failed send does not block the user.

Check the SMTP settings by sending yourself a test email (to `SMTP_USER`, or `-to` another address). Without credentials it stops and asks you to fill them in.

```bash
cd backend
go run ./cmd/test-email          # a plain test email
go run ./cmd/test-email -otp     # a sample verification code email
```

`.env` is git-ignored and kept out of the Docker images. Never commit real credentials.

## Demo accounts

These accounts also work on the **[live demo](https://your-demo-url.com)**.

A fresh database is filled with six demo accounts with posts, groups, events, chats and notifications, so every feature can be tried without registering or waiting for an email code. All of them are verified and share one password: **`Audit123!`**. The names are made up and the addresses use `example.com`, a domain reserved for examples.

| Name | Email | Profile | Try this |
|---|---|---|---|
| Maya Lind (`@maya`) | `maya@example.com` | Public | Creator of **Trail Runners**: accept **Sam**'s join request from the bell or the group's Requests tab. Has 3 unread chats (Leo, Nora, Iris) with emojis and flags. Her private post is visible only to Leo. |
| Leo Hart (`@leo`) | `leo@example.com` | Public | Sees Maya's private post (he is its chosen viewer) and Nora's private profile (he follows her). Has an **invitation to Book Club** from Nora in the bell. |
| Nora Vale (`@nora`) | `nora@example.com` | Private | Has a **follow request from Sam**: accept or decline it on her own profile or in the bell. Creator of **Book Club**, with a join request from **Iris**. |
| Sam Park (`@sam`) | `sam@example.com` | Private | Has a **follow request from Iris**; his own request to Nora is still waiting. Member of Book Club: answer the "Book night" event. |
| Iris Cole (`@iris`) | `iris@example.com` | Public | Member of Trail Runners ("Morning 10K" event, group chat). Her Book Club join request is waiting. Not a follower of Nora, so Nora's profile shows the private screen. |
| Theo Brandt (`@theo`) | `theo@example.com` | Public | The newcomer: follows only Maya. Has an **invitation to Trail Runners**. Try messaging Nora: not allowed, since neither follows the other. |

### Try it in 5 minutes

1. Log in as **Nora**, open your profile and accept **Sam**'s follow request.
2. Log in as **Maya**: the bell shows Sam's join request for Trail Runners, and Chats shows 3 unread conversations.
3. Log in as **Theo** in a second browser (or a private window): accept the Trail Runners invitation from the bell, open the group's **Events** tab and answer "Morning 10K". Then chat with Maya: messages arrive live in both windows.
4. Still as **Theo**: open **Nora**'s profile (private, no posts shown) and try to message her: not allowed.
5. Log in as **Leo**: the feed shows Maya's post "Only Leo can see this". As **Iris** it is not there.
6. As anyone: like and comment on a post, and write a post with an image, choosing "Private" and a few followers.

### How the demo data is added

The backend adds it on its first start, when the `users` table is empty: on a fresh clone with `docker compose up --build`, after `docker compose down -v`, or with a new database file when running without Docker. It never touches a database that already has accounts, and it never runs inside `go test`. The data and its images live in [`backend/pkg/db/seed`](backend/pkg/db/seed); the images are copied into the upload folder. Their sources and licenses are listed in [docs/image-credits.md](docs/image-credits.md). Dates are relative to the first start, so posts look recent and events are in the future.

- **Turn it off:** set `SEED_DEMO=0` (in `backend/.env`, the environment, or `SEED_DEMO=0 docker compose up --build`).
- **Remove it:** this deletes the six demo accounts and everything they created, and keeps every other account:

  ```bash
  cd backend && go run . -reset-demo                  # without Docker
  docker compose exec backend ./server -reset-demo    # with Docker
  ```

  If no accounts are left, the next start adds the demo data again, unless `SEED_DEMO=0`.

The shared password is for local testing only.

## Project structure

```text
social-network/
├── backend/
│   ├── server.go             entry point: settings, database, migrations, routes, shutdown
│   ├── cmd/test-email/       sends a test email to check the SMTP settings
│   └── pkg/
│       ├── db/seed/          demo accounts, posts and images, added to an empty database
│       ├── db/migrations/    numbered .up.sql / .down.sql files
│       ├── db/sqlite/        opens SQLite (foreign keys, WAL) and runs the migrations
│       ├── handlers/         all routes, one file per feature
│       ├── mail/             SMTP mailer, console mode and email templates
│       ├── middleware/       security headers, panic recovery, logging, CORS, auth
│       ├── models/           structs shared by the database and the JSON
│       ├── otp/              email codes: hashed, expiring, limited tries
│       ├── session/          sessions table and the session cookie
│       └── ws/               WebSocket hub: connections, pings, size and rate limits
├── frontend/
│   ├── app/                  pages: landing and feed, login, register, verify, forgot-password,
│   │                         profile, search, groups, chats, settings
│   ├── components/           navbar, post card, composer, chat window, notification bell, ...
│   ├── lib/                  API client, validation, hooks and their unit tests
│   ├── public/               self-hosted fonts and landing images
│   └── next.config.mjs       proxies /api, /uploads and /ws to the backend
├── docs/                     API contract, ERD, screenshots and image credits
└── docker-compose.yml        backend and frontend with named volumes
```

## Database

SQLite with 18 tables. The full diagram and the constraints are in [docs/erd.md](docs/erd.md).

| Area | Tables |
|---|---|
| Accounts | `users`, `sessions`, `otp_codes`, `password_reset_tokens` |
| Followers | `follows` (a pending row is a follow request) |
| Posts | `posts`, `post_viewers`, `post_likes`, `comments` |
| Groups | `groups`, `group_members`, `group_invites`, `group_requests`, `events`, `event_responses` |
| Chat | `messages`, `group_messages` |
| Notifications | `notifications` |

<a href="docs/erd.webp"><img src="docs/erd.webp" alt="Database diagram: the 18 tables and their keys"></a>

**Migrations.** Every table and column comes from a numbered migration in `backend/pkg/db/migrations/sqlite` (`NNNNNN_name.up.sql` and `.down.sql`), run with `golang-migrate`. The backend applies all pending migrations every time it starts.

To roll back every migration, run this from `backend/`. It drops all tables, so **all data is lost**, then exits:

```bash
go run . -migrate=down
```

The next `go run .` creates empty tables again. To get the shipped starting database back instead, stop the backend and delete the `backend/data` folder. A change to the schema always needs a new migration; never edit one that is already merged.

## API

The browser only talks to the frontend; Next.js forwards `/api/*`, `/uploads/*` and `/ws` to the backend. JSON uses `snake_case`, and every error looks like `{"error": "message"}`. The full contract is in [docs/api.md](docs/api.md).

| Area | Main endpoints |
|---|---|
| Health | `GET /api/health` |
| Auth | `POST /api/register`, `POST /api/login`, `POST /api/logout`, `GET /api/me`, `GET /api/nicknames/available` |
| Email verification | `POST /api/otp/send`, `POST /api/otp/verify`, `PATCH /api/me/email`, `DELETE /api/me` |
| Password reset | `POST /api/password/forgot`, `POST /api/password/verify-code`, `POST /api/password/reset` |
| Profiles | `GET /api/users?search=`, `GET /api/users/{id}`, `PATCH /api/me`, `PATCH /api/me/privacy` |
| Followers | `POST` / `DELETE /api/users/{id}/follow`, `GET /api/follow-requests`, `POST /api/follow-requests/{userId}/accept` or `/decline`, `GET /api/users/{id}/followers`, `GET /api/users/{id}/following` |
| Posts | `GET /api/feed`, `GET /api/users/{id}/posts`, `POST /api/posts`, `GET` / `DELETE /api/posts/{id}`, `POST` / `DELETE /api/posts/{id}/like` |
| Comments | `GET` / `POST /api/posts/{id}/comments`, `DELETE /api/posts/{id}/comments/{commentId}` |
| Groups | `GET` / `POST /api/groups`, `GET /api/groups/{id}`, invitations (`/invite`, `/api/group-invites/{id}/...`), join requests (`/request`, `/requests`, `/api/group-requests/{id}/...`), `GET` / `POST /api/groups/{id}/posts` |
| Events | `GET` / `POST /api/groups/{id}/events`, `POST /api/events/{id}/respond` |
| Chat history | `GET /api/conversations`, `GET /api/chat-contacts`, `GET /api/messages/{userId}`, `POST /api/messages/{userId}/read`, `GET /api/messages/unread-count`, `GET /api/groups/{id}/messages` |
| Notifications | `GET /api/notifications`, `GET /api/notifications/unread-count`, `POST /api/notifications/{id}/read`, `POST /api/notifications/read-all` |
| Images | `GET /uploads/{file}` |

**WebSocket `/ws`.** One authenticated connection per tab. Every frame is a JSON envelope: `{"type", "request_id", "payload"}`.

| Direction | Event types |
|---|---|
| Browser to server | `private_message`, `group_message`, `typing` |
| Server to browser | `private_message`, `group_message`, `typing`, `messages_read`, `notification`, `notification_removed`, `group_changed`, `post_created`, `comment_created`, `post_likes`, `post_deleted`, `comment_deleted`, `error` |

## Security and privacy

- **Sessions:** a random UUID in the `session_id` cookie (HttpOnly, SameSite=Lax, 7 days, `Secure` with `COOKIE_SECURE=1`). The session lives in the database; logout deletes it, and a password reset deletes every session of the user.
- **Passwords:** stored as bcrypt hashes, never in plain text. The strong-password rules apply at sign-up and at reset.
- **Codes and tokens:** email codes are stored as salted SHA-256 hashes. They expire after 10 minutes and allow 5 tries. Reset tokens are hashed, single use and valid for 15 minutes.
- **Access rules in the backend:** profile privacy, post privacy (one SQL condition used by every query that returns posts), members-only groups and the chat follow rule are all checked on the server. The chat and like rules are checked inside the `INSERT` itself. Something you may not see answers `404`, exactly like something that does not exist.
- **Uploads:** the type is detected from the file bytes (JPEG, PNG, GIF or WebP), images are capped at 50 MB, and files get random UUID names. They are served only to logged-in users, and post images only to users who may see the post. There is no folder listing.
- **Rate limits:** one email code per 60 seconds; 5 password reset requests per hour per email address and per client; WebSocket frames are limited to 16 KB and 10 per second per connection.
- **Headers and origins:** every response sends `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY` and `Referrer-Policy: strict-origin-when-cross-origin`. The WebSocket only accepts the browser origins in `FRONTEND_ORIGIN`.
- **Logs:** the backend log prints each password reset code on purpose, for the audit.

## Testing

Backend (from `backend/`):

```bash
go test ./...
go test -race ./...
go vet ./...
```

Frontend (from `frontend/`):

```bash
npm ci
npm test
npm run build
```

The backend tests cover the demo data, validation, password reset, email codes, unverified accounts, uploads, sessions, middleware, mail and the WebSocket hub. The frontend tests (`lib/*.test.mjs`) check the same validation cases as `backend/pkg/handlers/validation_test.go`, so the register page and the server always agree. Change both when a rule changes.

## Team

| Name | Username |
|---|---|
| Ahmed Ramadhan | aramadhan |
| Abdulla Homaidan | ahomaida |
| Abbas Al Abdulwahab | aalabdulw |
| Hussain Madan | humadan |
| Jassim Thamer | jthamer |
