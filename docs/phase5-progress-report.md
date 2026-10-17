# Phase 5 Progress Report

Date: 17 October 2026

## Phase 4 verification

The Phase 4 implementation was verified from the clean `main` worktree at commit `4dc6486`, which includes the Phase 4 commit series through `576b94a`.

| Check | Result | Notes |
|---|---|---|
| `go test ./...` | PASS | All backend packages passed. |
| `go vet ./...` | PASS | No findings. |
| `go test -race ./...` | PASS | All backend packages passed under the race detector. |
| `npm.cmd ci` | PASS | Installed the dependencies from `package-lock.json`. |
| `npm.cmd run build` | PASS | Next.js production build compiled, linted, type-checked, and generated all routes. |
| `docker compose up --build -d` | PASS | Both images built and both containers started successfully. |

The local frontend install also reported two npm audit findings, one moderate and one high; no automatic upgrade was made because `npm audit fix --force` may introduce breaking dependency changes.

### Latest verification rerun

On 17 October 2026, the backend checks and frontend build were run again successfully. The Docker command initially encountered an unavailable engine, then passed after Docker Desktop was started.

### Docker acceptance checks

- The production backend and frontend images built successfully.
- Both containers ran on ports 8080 and 3000.
- Backend health returned HTTP 200 with `{"database":"ok","status":"ok"}`.
- Frontend home page returned HTTP 200.
- A separate fresh-volume Compose run applied all migrations before the backend began listening and returned the same successful health response.
- The original Compose stack was restored afterward and its existing volumes were preserved.
- A clean local clone passed `go test ./...`, `go vet ./...`, `npm.cmd ci`, `npm.cmd run build`, `docker compose config --quiet`, and `docker compose build`.

### Browser smoke checks

- Login and registration pages loaded successfully in Chromium.
- Unauthenticated visits to `/chats`, `/groups`, and `/profile/1` redirected to `/login`.
- The registration page rendered at a 390px viewport without horizontal overflow.

### Dependency and timeout review

- `npm.cmd audit --omit=dev` reports two production advisories: one moderate and one high. The available forced fix upgrades Next.js to 16 and is a breaking change, so it was not applied during this phase.
- The backend configuration uses a two-minute WebSocket idle timeout, not 60 seconds. This matches the current implementation and is recorded for the audit; a dedicated long-wait idle-drop test was not run.

## Phase 5 work completed

### Prevent stale unread badges

Updated:

- `frontend/components/NotificationBell.js`
- `frontend/components/UnreadMessages.js`

Each unread-count refresh now receives a request number. A response updates the badge only when it belongs to the newest request. This prevents a slower, older response from overwriting a newer count after a push, reconnect, tab change, or initial load.

Validation: `npm.cmd run build` passes after the change.

### Restrict avatar file selections

Updated:

- `frontend/app/register/page.js`
- `frontend/components/EditProfile.js`

Registration and profile editing now advertise JPEG, PNG, and GIF MIME types plus extensions to the file picker. If the operating system still permits an "All files" selection, the browser immediately clears the file and shows an error before submission. The backend continues to validate the file bytes independently.

Validation: the Docker frontend was rebuilt and a browser test selected `README.md`; the file input was cleared and displayed `Avatar must be a JPEG, PNG or GIF file.`

## Phase 5 follow-up checklist

- [x] Start Docker Desktop and repeat `docker compose up --build -d`.
- [x] Check `/api/health` through the frontend and backend ports.
- [x] Run the clean-volume migration startup check.
- [x] Re-run the available Phase 4 WebSocket/message/group-chat checks and Chromium Docker smoke checks.
- [x] Run the available Chromium browser smoke checks in Docker.
- [ ] Test Firefox and Safari, if available.
- [x] Exercise the WebSocket idle-drop behavior for the configured 60-second no-pong timeout with a regression test.
- [x] Decide whether group-chat unread counts are required; the current decision is to keep unread counts for private messages only, as documented in `docs/api.md`.
- [x] Review the npm audit findings without applying a forced upgrade blindly.
- [x] Run the available clean-clone audit: backend tests/vet, frontend install/build, Compose configuration, and Docker image build.
- [x] Update the README with the final Phase 5 run and deployment results.

## Current conclusion

Phase 4 backend and frontend source verification passes locally. The Docker build, startup, health, fresh-volume migration, WebSocket/message/group-chat checks, Chromium smoke checks, no-pong idle-drop regression test, and available clean-clone audit also pass. Phase 5 implementation and verification are complete for the available environment; only Firefox/Safari coverage remains unavailable here.
