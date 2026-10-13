# Database ERD

Every table is created by a numbered migration in
`backend/pkg/db/migrations/sqlite`. `users`, `sessions`, `follows`, `notifications`, `posts`
and `post_viewers` exist so far; the rest are added by the phase that needs them.

| Table | Migration added in |
|---|---|
| users, sessions | Phase 0 |
| follows | Phase 1 (migration 000003: the table only, so private profiles can check it. The follow endpoints are Phase 2) |
| posts, post_viewers | Phase 2 (migrations 000005, 000006) |
| comments | Phase 2 |
| notifications | Phase 2 (migration 000004) |
| groups, group_members, group_invites, group_requests, events, event_responses | Phase 3 |
| messages, group_messages | Phase 4 |

**Follow requests** are rows in `follows` with `status = 'pending'`, as in P2's
week-2 spec. There is no separate `follow_requests` table. Accepting a request
flips the row to `accepted`. Declining or cancelling deletes the row, so the
user can request again.

```mermaid
erDiagram
    users {
        INTEGER id PK
        TEXT email UK
        TEXT password_hash
        TEXT first_name
        TEXT last_name
        TEXT date_of_birth
        TEXT avatar_path
        TEXT nickname
        TEXT about_me
        INTEGER is_public "1 public, 0 private"
        DATETIME created_at
    }
    sessions {
        TEXT id PK "uuid"
        INTEGER user_id FK
        DATETIME expires_at
        DATETIME created_at
    }
    follows {
        INTEGER follower_id PK,FK
        INTEGER following_id PK,FK
        TEXT status "pending | accepted"
        DATETIME created_at
    }
    posts {
        INTEGER id PK
        INTEGER user_id FK "author"
        TEXT content "may be empty if there is an image"
        TEXT image_path "may be empty if there is text"
        TEXT privacy "public | almost_private | private"
        DATETIME created_at
    }
    post_viewers {
        INTEGER post_id PK,FK
        INTEGER user_id PK,FK
    }
    comments {
        INTEGER id PK
        INTEGER post_id FK
        INTEGER user_id FK
        TEXT content
        TEXT image_path
        DATETIME created_at
    }
    groups {
        INTEGER id PK
        INTEGER creator_id FK
        TEXT title
        TEXT description
        DATETIME created_at
    }
    group_members {
        INTEGER group_id PK,FK
        INTEGER user_id PK,FK
        DATETIME joined_at
    }
    group_invites {
        INTEGER id PK
        INTEGER group_id FK
        INTEGER inviter_id FK
        INTEGER invitee_id FK
        DATETIME created_at
    }
    group_requests {
        INTEGER id PK
        INTEGER group_id FK
        INTEGER user_id FK
        DATETIME created_at
    }
    events {
        INTEGER id PK
        INTEGER group_id FK
        INTEGER creator_id FK
        TEXT title
        TEXT description
        DATETIME event_time
        DATETIME created_at
    }
    event_responses {
        INTEGER event_id PK,FK
        INTEGER user_id PK,FK
        TEXT response "going | not_going"
    }
    messages {
        INTEGER id PK
        INTEGER sender_id FK
        INTEGER receiver_id FK
        TEXT content
        INTEGER is_read
        DATETIME created_at
    }
    group_messages {
        INTEGER id PK
        INTEGER group_id FK
        INTEGER sender_id FK
        TEXT content
        DATETIME created_at
    }
    notifications {
        INTEGER id PK
        INTEGER user_id FK "recipient"
        INTEGER actor_id FK "who caused it, nullable"
        TEXT type "checked in Go, no CHECK, so Phase 3 can add types"
        INTEGER ref_id "NULL for follow types; group / event id in Phase 3"
        INTEGER is_read
        DATETIME created_at
    }

    users ||--o{ sessions : has
    users ||--o{ follows : "follower"
    users ||--o{ follows : "following"
    users ||--o{ posts : writes
    posts ||--o{ post_viewers : "visible to"
    users ||--o{ post_viewers : "chosen viewer"
    posts ||--o{ comments : has
    users ||--o{ comments : writes
    users ||--o{ groups : creates
    groups ||--o{ group_members : has
    users ||--o{ group_members : "member of"
    groups ||--o{ group_invites : has
    groups ||--o{ group_requests : has
    groups ||--o{ posts : "contains (Phase 3 adds posts.group_id)"
    groups ||--o{ events : has
    events ||--o{ event_responses : has
    users ||--o{ event_responses : answers
    users ||--o{ messages : sends
    groups ||--o{ group_messages : has
    users ||--o{ notifications : receives
```

## Constraints to remember

- Every child table uses `ON DELETE CASCADE` toward `users`, `posts`, `groups` and `events`.
- `UNIQUE (group_id, invitee_id)` on `group_invites` and `UNIQUE (group_id, user_id)` on `group_requests` block duplicates.
- `CHECK (follower_id <> following_id)` on `follows`, so nobody can follow themselves.
- Index every foreign key that is used in a `WHERE`, e.g. `follows(following_id, status)`, `posts(user_id)`, `posts(group_id)`, `messages(sender_id, receiver_id)`.
