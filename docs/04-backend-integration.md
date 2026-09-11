# Backend Integration Guide

This document describes the backend currently wired into the app, plus the remaining production integrations.

## Current Database Schema (SQLite)

```sql
CREATE TABLE documents (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
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
    uploaded_by    INTEGER,
    created_at     TEXT NOT NULL
);

CREATE VIRTUAL TABLE documents_fts USING fts5(
    title, title_np, description, body_text,
    content='documents', content_rowid='id',
    tokenize='unicode61'
);

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer'
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id),
    expires_at TEXT NOT NULL
);
```

FTS triggers keep `documents_fts` synchronized after document inserts, updates, and deletes. Migrations are tracked with `PRAGMA user_version`.

## Handler Map

| Route | Handler | Backing behavior |
|-------|---------|------------------|
| `GET /` | `indexHandler` | `queryDocuments(q, cat)` reads SQLite and FTS5 |
| `GET /document/{id}` | `documentHandler` | `getDocumentByID(id)` reads SQLite |
| `GET /document/{id}/edit` | `editGetHandler` | admin-gated document edit form |
| `POST /document/{id}/edit` | `editPostHandler` | admin-gated SQLite update |
| `GET /upload` | `uploadGetHandler` | uploader-gated upload form |
| `POST /upload` | `uploadPostHandler` | uploader-gated multipart parse, local file save, SQLite insert |
| `GET /dashboard` | `dashboardHandler` | admin-gated `queryDocuments(q, cat)` |
| `POST /login` | `loginPostHandler` | bcrypt password check and session creation |
| `POST /logout` | `logoutHandler` | session delete and cookie clear |

## File Upload Flow

```text
POST /upload
  - r.ParseMultipartForm(50 << 20)    // 50 MB limit
  - validate fields (title required)
  - r.FormFile("file") -> save to /static/docs/
  - INSERT INTO documents (title, title_np, category, description,
      body_html, body_text, lang, script,
      orig_author, orig_author_np, orig_year, orig_month, orig_day,
      file_path, uploaded_by, created_at)
  - http.Redirect -> /document/{new_id}
```

The current implementation stores development uploads locally under `/static/docs/`. For production, replace that block with object storage and keep `document.FilePath` as a browser-accessible URL, such as `https://cdn.yogilib.com/docs/treaty-1815.pdf`.

## Authentication

Authentication is live. Sessions are stored in SQLite and exposed through an `HttpOnly` cookie named `session`.

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

// Protect routes:
mux.HandleFunc("GET /upload", requireRole("uploader", uploadGetHandler))
mux.HandleFunc("GET /dashboard", requireRole("admin", dashboardHandler))
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

Current implementation: SQLite FTS5 with prefix matching.

```sql
SELECT d.*
FROM documents d
WHERE d.id IN (
    SELECT rowid FROM documents_fts WHERE documents_fts MATCH ?
)
ORDER BY d.created_at DESC;
```

The Go layer appends `*` to the submitted query for prefix matching and can combine search with category filtering.

## Remaining Production Work

- Replace local `/static/docs/` file storage with R2, S3, or another object store.
- Remove or rotate seeded development credentials before deploying.
- Decide whether excerpts, store items, mission content, and similar-site content should remain static or move into database tables.
- Add contributor display by joining `documents.uploaded_by` to `users`.
