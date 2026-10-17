# Arabic Demo Dataset Report

Date: 17 October 2026

This report describes the development-only data seeded into the running Docker database. It was created through the authenticated API and does not change application code or migrations.

## Shared test password

All ten accounts use the same development password:

`DemoPass123!`

The application logs in with email, not a separate username.

## Accounts

| ID | Name | Nickname | Login email | Profile | Avatar | Profile posts |
|---:|---|---|---|---|---|---:|
| 2 | Amal Haddad | amal | `arabic-demo-1@social-network.local` | Public | Visible AH initials PNG | 2 |
| 3 | Noor Khalil | noor | `arabic-demo-2@social-network.local` | Private | Visible NK initials PNG | 1 |
| 4 | Omar Mansour | omar | `arabic-demo-3@social-network.local` | Public | Visible OM initials PNG | 1 |
| 5 | Layla Nasser | layla | `arabic-demo-4@social-network.local` | Private | Visible LN initials PNG | 1 |
| 6 | Yara Fahmi | yara | `arabic-demo-5@social-network.local` | Public | Visible YF initials PNG | 1 |
| 7 | Zaid Qasim | zaid | `arabic-demo-6@social-network.local` | Private | Visible ZQ initials PNG | 1 |
| 8 | Hana Darwish | hana | `arabic-demo-7@social-network.local` | Public | Visible HD initials PNG | 1 |
| 9 | Rami Saleh | rami | `arabic-demo-8@social-network.local` | Private | Visible RS initials PNG | 1 |
| 10 | Mariam Saad | mariam | `arabic-demo-9@social-network.local` | Public | Visible MS initials PNG | 1 |
| 11 | Tariq Aziz | tariq | `arabic-demo-10@social-network.local` | Private | Visible TA initials PNG | 1 |

## Follow relationships

Accepted relationships:

- Amal -> Noor
- Yara -> Zaid
- Mariam -> Tariq
- Noor -> Amal
- Layla -> Yara
- Zaid -> Hana
- Rami -> Mariam
- Tariq -> Amal

Pending follow requests:

- Omar -> Layla
- Hana -> Rami

The pending requests are intentionally left unanswered so the follow-request UI and notification flow can be tested.

## Posts

The dataset contains 13 profile posts:

- 10 public or almost-private demo posts
- 5 almost-private posts from private-profile users
- 3 private posts with selected viewers
- Several posts include a PNG image upload
- Posts use Arabic-style account names, transliterated nicknames, and privacy-specific content. Nicknames are transliterated because the backend requires ASCII nicknames.

Private post viewers were selected from accepted follower relationships so the private-post access rules can be tested from both an allowed and blocked account.

## Verified after seeding

- All 10 accounts log in successfully.
- All 10 accounts have an avatar path and the avatar endpoint returns the image.
- 5 profiles are public and 5 are private.
- 2 pending follow requests are visible to the intended private-profile owners.
- Amal's feed returns only the posts visible to Amal under the privacy rules.
- Backend health returns `{"database":"ok","status":"ok"}`.

## Reset

This data is stored in the Docker named volume. To remove it and recreate an empty database:

```sh
docker compose down -v
docker compose up --build
```

These credentials are for local development only and must not be used in production.
