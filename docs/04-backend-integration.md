# Backend Integration Guide

This document describes the Neon Postgres backend currently wired into the app, plus the remaining production integrations.

## Current Database Schema (Neon Postgres)

```sql
CREATE TABLE documents (
    id             BIGSERIAL PRIMARY KEY,
    title          TEXT NOT NULL,
    title_np       TEXT,
    category       TEXT,
    description    TEXT,
    body_html      TEXT,
    body_text      TEXT,
    lang           TEXT,
    script         TEXT,
    orig_author    TEXT,
    orig_author_np TEXT,
    orig_year      TEXT,
    orig_month     TEXT,
    orig_day       TEXT,
    file_path      TEXT,
    uploaded_by    BIGINT,
    created_at     TEXT NOT NULL
);

CREATE INDEX documents_search_idx ON documents
USING GIN (to_tsvector('simple',
    COALESCE(title, '') || ' ' ||
    COALESCE(title_np, '') || ' ' ||
    COALESCE(description, '') || ' ' ||
    COALESCE(body_text, '')
));

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer'
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    expires_at TEXT NOT NULL
);
```

The app creates these tables and the search index automatically at startup. For larger production changes, move schema changes into a dedicated migration tool and run them with `DATABASE_URL_UNPOOLED`.

## Neon Project

This workspace is linked to Neon project `lucky-boat-33662130`, branch `production`. The local `.neon` file and `.env.local` file are intentionally ignored by Git because they contain local machine state and secrets.

On another computer:

```bash
neon link --project-id lucky-boat-33662130 --branch production -y
go run main.go
```

## Handler Map

| Route | Handler | Backing behavior |
|-------|---------|------------------|
| `GET /` | `indexHandler` | `queryDocuments(q, cat)` reads Neon Postgres |
| `GET /document/{id}` | `documentHandler` | `getDocumentByID(id)` reads Neon Postgres |
| `GET /document/{id}/edit` | `editGetHandler` | admin-gated document edit form |
| `POST /document/{id}/edit` | `editPostHandler` | admin-gated Postgres update |
| `GET /upload` | `uploadGetHandler` | uploader-gated upload form |
| `POST /upload` | `uploadPostHandler` | uploader-gated multipart parse, local file save, Postgres insert |
| `GET /dashboard` | `dashboardHandler` | admin-gated `queryDocuments(q, cat)` |
| `POST /login` | `loginPostHandler` | bcrypt password check and session creation |
| `POST /logout` | `logoutHandler` | session delete and cookie clear |

## File Upload Flow

```text
POST /upload
  - r.ParseMultipartForm(50 << 20)    // 50 MB limit
  - validate fields (title required)
  - r.FormFile("file") -> save to /static/docs/
  - INSERT INTO documents (...) RETURNING id
  - http.Redirect -> /document/{new_id}
```

The database is shared now, but uploaded files are still local to whichever computer handled the upload. For production or true multi-device uploads, replace local `/static/docs/` storage with object storage such as Neon Object Storage, Cloudflare R2, or S3.

## Authentication

Authentication is live. Sessions are stored in Postgres and exposed through an `HttpOnly` cookie named `session`.

```go
func requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        user := sessionUser(r)
        if user == nil || !hasRole(user.Role, role) {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }
        next(w, r)
    }
}
```

Development seed users are created on the first run only:

| Email | Password | Role |
|-------|----------|------|
| `admin@yogilib.org` | `admin123` | `admin` |
| `upload@yogilib.org` | `upload123` | `uploader` |

## Contributor Attribution

`uploadPostHandler` reads the current session user and stores `uploaded_by` with the inserted document. `documentHandler` does not yet join to `users` to display the contributor.

Original author (`orig_author`, `orig_author_np`) is a separate field for the historical author of the document, such as `Yogi Narharinath / योगी नरहरिनाथ`.

## Search

Current implementation: Postgres full-text search plus `ILIKE` fallback matching.

```sql
SELECT d.*
FROM documents d
WHERE (
    to_tsvector('simple',
        COALESCE(d.title, '') || ' ' ||
        COALESCE(d.title_np, '') || ' ' ||
        COALESCE(d.description, '') || ' ' ||
        COALESCE(d.body_text, '')
    ) @@ plainto_tsquery('simple', $1)
    OR d.title ILIKE $2
    OR d.title_np ILIKE $2
    OR d.description ILIKE $2
    OR d.body_text ILIKE $2
)
ORDER BY d.created_at DESC;
```

## Remaining Production Work

- Replace local `/static/docs/` file storage with shared object storage.
- Remove or rotate seeded development credentials before public deployment.
- Move schema management into a formal migration system.
- Decide whether excerpts, store items, mission content, and similar-site content should remain static or move into database tables.
- Add contributor display by joining `documents.uploaded_by` to `users`.
