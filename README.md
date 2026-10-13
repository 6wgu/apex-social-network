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

Requires Go 1.24+ with a C compiler (CGO is needed for SQLite) and Node.js 20+.

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
| `FRONTEND_ORIGIN` | `http://localhost:3000` (CORS, only used when the frontend calls `:8080` directly) |

## Migrations

Migrations live in `backend/pkg/db/migrations/sqlite` as `NNNNNN_name.up.sql` / `.down.sql`. All pending ones are applied on every start.

```sh
cd backend
go run . -migrate=down    # roll back every migration and exit
go run .                  # apply them again and start
go test ./pkg/db/sqlite   # automated check: fresh DB -> up -> down -> up
```

Rule: a new table or column means a new numbered migration. Never edit a migration that has already been merged.

## Project structure

```
backend/
├── server.go                  entry point: config, DB, migrations, middleware, server
└── pkg/
    ├── db/
    │   ├── migrations/sqlite/ numbered .up.sql / .down.sql files
    │   └── sqlite/sqlite.go   connection (FKs, WAL, busy timeout) + migrate up/down
    ├── handlers/              routes and one file per feature
    ├── middleware/            logging, panic recovery, CORS
    ├── models/                structs shared by DB and JSON
    └── session/               sessions and cookies (Phase 1)
frontend/
├── app/                       Next.js pages (App Router)
├── components/                Navbar, etc.
├── lib/api.js                 fetch wrapper (credentials, JSON errors, 401 → /login)
└── next.config.mjs            proxies /api, /uploads, /ws to the backend
```
