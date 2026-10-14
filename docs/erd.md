# Database ERD

Every table is created by a numbered migration in
`backend/pkg/db/migrations/sqlite`. Every table of Phases 0 to 3 exists; `messages` and
`group_messages` are added by Phase 4.

| Table | Migration added in |
|---|---|
| users, sessions | Phase 0 |
| follows | Phase 1 (migration 000003: the table only, so private profiles can check it. The follow endpoints are Phase 2) |
| posts, post_viewers | Phase 2 (migrations 000005, 000006) |
| comments | Phase 2 (migration 000007) |
| notifications | Phase 2 (migration 000004) |
| groups, group_members, group_invites, group_requests, events, event_responses | Phase 3 (migrations 000008 to 000013) |
| posts.group_id | Phase 3 (migration 000014: `ALTER TABLE posts ADD COLUMN`, so no merged migration is edited) |
| messages, group_messages | Phase 4 |

**Follow requests** are rows in `follows` with `status = 'pending'`, as in P2's
week-2 spec. There is no separate `follow_requests` table. Accepting a request
flips the row to `accepted`. Declining or cancelling deletes the row, so the
user can request again.

**Group invitations and join requests** work the same way: a row exists only
while it is pending, and answering (or cancelling a request) deletes it.
Joining either way also deletes the other one, so a member never has a stale
invitation or request. The creator is a row in `group_members` too.

**Group posts** are rows in `posts` with `group_id` set. Their `privacy` column
is stored as `public` but ignored: the group's members see them, nobody else.
Profile posts have `group_id` NULL.

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
        INTEGER group_id FK "NULL for profile posts"
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
        DATETIME responded_at
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
        INTEGER actor_id FK "who caused it; rows are deleted with that user (CASCADE)"
        TEXT type "checked in Go, no CHECK, so Phase 3 can add types"
        INTEGER ref_id "NULL for follow types; group id, or event id for group_event"
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
    groups ||--o{ posts : "contains (posts.group_id)"
    groups ||--o{ events : has
    events ||--o{ event_responses : has
    users ||--o{ event_responses : answers
    users ||--o{ messages : sends
    groups ||--o{ group_messages : has
    users ||--o{ notifications : receives
```

## Constraints to remember

- Every child table uses `ON DELETE CASCADE` toward `users`, `posts`, `groups` and `events`.
- `UNIQUE (group_id, invitee_id)` on `group_invites` and `UNIQUE (group_id, user_id)` on `group_requests` block duplicates; `PRIMARY KEY (event_id, user_id)` on `event_responses` keeps one answer per member, and answering again updates it.
- `CHECK (title <> '')` and `CHECK (description <> '')` on `groups` and `events`; `CHECK (response IN ('going', 'not_going'))` on `event_responses`.
- `CHECK (follower_id <> following_id)` on `follows`, so nobody can follow themselves.
- Index every foreign key that is used in a `WHERE`, e.g. `follows(following_id, status)`, `posts(user_id)`, `posts(group_id)`, `messages(sender_id, receiver_id)`.
