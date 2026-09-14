# Yogilib — Web Frontend

A digital archive for the works of **Yogi Narharinath** (योगी नरहरिनाथ) — Nepali scholar, historian, and religious figure. The site lets visitors browse, search, and read historical documents in English, Nepali, and Sanskrit, and lets contributors upload new material.

Built with **Go** using `net/http` and `html/template`. Every page is server-rendered. The shared database is **Neon Postgres**, so the same project data can be used from multiple computers.

---

## Quick start

```bash
go mod tidy             # install Go dependencies
go run main.go          # dev server at http://localhost:8080
DEV_RELOAD=1 go run main.go # dev server with browser auto-reload
go build -o yogilib .   # production binary
PORT=9000 ./yogilib     # custom port
```

Requires **Go 1.25+** and a `DATABASE_URL`. The app automatically reads `.env.local` when present; Neon writes that file during `neon link`, `neon checkout`, and `neon deploy`.

To request changes from your phone and continue on another computer, start with the [cross-device guide](docs/07-cross-device-work.md).

On a second computer:

```bash
git clone git@github.com:helonmelon/yogilib.git
cd yogilib
neon link --project-id lucky-boat-33662130 --branch production -y
go run main.go
```

If you are not using the Neon CLI, copy `.env.example` to `.env.local` and fill in `DATABASE_URL`.

`DEV_RELOAD=1` enables a development-only watcher for templates, reader CSS, and reader JavaScript. Ordinary runs do not poll for changes.

---

## What's in the box

| Area | Status |
|---|---|
| All public pages (home, about, works, document viewer, excerpts, store, similar sites) | Done |
| Neon Postgres database with full-text search | Done |
| Login system with session-based auth and role tiers | Done |
| Contribute / upload form | Done |
| Admin pages (dashboard, edit) | Done |
| Document reader controls, typography, navigation, passage links, and responsive original/transcription view | Done |
| Private reader notes and admin revision history | Done |
| Development browser auto-reload (`DEV_RELOAD=1`) | Done |
| Preeti → Unicode converter (legacy Nepali encoding) | Done |
| ITRANS → Devanagari converter (Sanskrit transliteration) | Done |
| File / object storage (R2, S3, Neon Object Storage) | Not connected |

---

## Stack

```text
Browser -> net/http Mux -> Auth Middleware -> Handler -> html/template -> HTML
                                |
                           Neon Postgres
```

- **Server**: `net/http` + `html/template`
- **Database**: Neon Postgres via `github.com/jackc/pgx/v5`
- **Search**: Postgres full-text search with `to_tsvector('simple', ...)` plus `ILIKE` fallback
- **Auth**: bcrypt passwords (`golang.org/x/crypto`), session tokens in Postgres, `HttpOnly` cookie
- **Styles**: single `static/css/style.css`, Himalaya font for Devanagari
- **Language support**: Unicode Devanagari, Preeti and ITRANS converters for legacy text

---

## Authentication & access tiers

The site is **publicly readable** — no login needed to browse documents, excerpts, or any reading page. Login is only required to contribute or administrate.

### Roles

| Role | Access |
|---|---|
| *(public)* | All reading pages — home, documents, excerpts, about, works, store |
| `uploader` | Everything public + `/upload` |
| `admin` | Everything above + `/dashboard`, `/document/{id}/edit` |

### Seeded accounts (development only)

On first run against an empty database, two development accounts are created automatically:

| Email | Password | Role |
|---|---|---|
| `admin@yogilib.org` | `admin123` | `admin` |
| `upload@yogilib.org` | `upload123` | `uploader` |

**Change these before deploying to production.**

### Sessions

- Sessions are stored in the `sessions` table in Postgres
- A random 64-character token is set as an `HttpOnly` cookie named `session`
- Sessions expire after **30 days**
- Logging out deletes the session from the database and clears the cookie

---

## Database

The app expects `DATABASE_URL` to point at Neon Postgres. Schema is created automatically on startup if the tables do not exist.

### Schema overview

```sql
documents (id, title, title_np, category, description, body_html, body_text, lang, script, orig_author, orig_author_np, orig_year, orig_month, orig_day, file_path, uploaded_by, created_at)
users (id, email, password_hash, role)
sessions (token, user_id, expires_at)
```

### Search

Search indexes title, Nepali title, description, and body text through a Postgres GIN expression index. Queries use Postgres full-text search plus `ILIKE` matching so short Nepali/English snippets still work naturally.

---

## Docs

Document pages include a clickable table of contents, adjustable reading settings, in-document search, copyable passage links, responsive original/transcription viewing, private signed-in notes, and admin revision restore. Reading preferences are stored in the browser. Uploaded files remain local under `static/docs` until shared object storage is connected.

For repeatable work, prefer deterministic Go code and small scripts. Use AI for design exploration or unfamiliar problems when useful, then keep changes reviewable and testable. Bundle related work and run targeted validation to conserve resources.

| File | What it covers |
|---|---|
| [`01-getting-started.md`](docs/01-getting-started.md) | Running the server, project layout, adding new pages |
| [`02-architecture.md`](docs/02-architecture.md) | Request lifecycle, core types, routing table, template system |
| [`03-design-system.md`](docs/03-design-system.md) | Typography, colour palette, UI components |
| [`04-backend-integration.md`](docs/04-backend-integration.md) | DB schema, handler map, file upload flow, search |
| [`05-language-support.md`](docs/05-language-support.md) | Unicode, Nepali typing, Preeti encoding, ITRANS, Sanskrit |
| [`06-changelog.md`](docs/06-changelog.md) | Change history |
