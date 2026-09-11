# Architecture

## Overview

Yogilib is a **pure Go stdlib web server** — no frameworks, no build pipeline, no JavaScript bundler. Every page is server-rendered via Go's `html/template`. Static assets (CSS, fonts, images, JS helpers) are served directly from the `static/` directory.

```
Browser → net/http Mux → Auth Middleware → Handler → html/template → HTML response
                                             ↓
                                        SQLite + FTS5
```

## Request Lifecycle

1. Browser sends `GET /upload`
2. `mux.HandleFunc("GET /upload", requireRole("uploader", uploadGetHandler))` matches
3. `requireRole` checks the session cookie and confirms the user is an uploader or admin
4. `uploadGetHandler` populates a `PageData` struct
5. `render(w, r, "upload", data)` parses `base.html` + `upload.html`
6. `base.html` renders the shell; `upload.html` fills the `{{template "content" .}}` slot
7. Response is streamed to the browser

## Core Types (`main.go`)

### `PageData`
The single data envelope passed to every template. Only populate the fields relevant to each page — unused fields are zero-valued.

```go
type PageData struct {
    Title      string
    Query      string       // search query string
    Category   string       // active category filter
    Categories []string     // list of category tab labels
    Flash      string       // success/info banner
    Error      string       // error banner
    Documents  []Document
    Doc        *Document    // single document view
    Excerpts   []Excerpt
    Exc        *Excerpt     // single excerpt view
    StoreItems []StoreItem
    User       *User        // nil when not logged in
}
```

### `Document`
```go
type Document struct {
    ID           string
    Title        string       // English title
    TitleNP      string       // Nepali title (Unicode Devanagari)
    Category     string       // किताब | कागजात | रेकर्ड | पत्रिका | अंश | अन्य
    Description  string
    BodyHTML     template.HTML
    BodyText     string       // plain text copy used by search
    Lang         string
    Script       string
    OrigAuthor   string
    OrigAuthorNP string
    OrigYear     string
    OrigMonth    string
    OrigDay      string
    FilePath     string       // local development URL, e.g. /static/docs/file.pdf
    UploadedBy   int
    CreatedAt    string       // formatted display date
}
```

### `Excerpt`
```go
type Excerpt struct {
    Slug  string
    Title string
    Body  template.HTML    // stored as HTML; marked safe before passing
}
```

## Routing Table

| Method | Route | Handler | Auth |
|--------|-------|---------|------|
| GET | `/` | `indexHandler` | Public |
| GET | `/about` | `aboutHandler` | Public |
| GET | `/works` | `worksHandler` | Public |
| GET | `/document/{id}` | `documentHandler` | Public |
| GET | `/document/{id}/edit` | `editGetHandler` | Admin |
| POST | `/document/{id}/edit` | `editPostHandler` | Admin |
| GET | `/upload` | `uploadGetHandler` | Uploader+ |
| POST | `/upload` | `uploadPostHandler` | Uploader+ |
| GET | `/dashboard` | `dashboardHandler` | Admin |
| GET | `/login` | `loginGetHandler` | Public |
| POST | `/login` | `loginPostHandler` | Public |
| POST | `/logout` | `logoutHandler` | Public |

`requireRole` enforces role tiers: `viewer`, `uploader`, then `admin`.

## Template System

- **`base.html`** defines `{{define "base"}}` — the full page shell (DOCTYPE, `<head>`, nav, footer).
- Every other template defines `{{define "content"}}` — only the unique body of that page.
- `render()` in `main.go` always parses both `base.html` and the page template together, then executes `"base"`.

### Template Variables

All data flows through `PageData`. Access in templates with `{{.FieldName}}`. Example:

```html
{{if .Flash}}
<div class="flash flash-ok">{{.Flash}}</div>
{{end}}

{{range .Documents}}
<p>{{.Title}} — {{.TitleNP}}</p>
{{end}}
```

## Static Assets

Served by `http.FileServer` under `/static/`:

```go
mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
```

| Path | Contents |
|------|----------|
| `/static/css/style.css` | All site CSS |
| `/static/fonts/` | Himalaya woff/woff2 |
| `/static/imgs/` | Yogi Narharinath photos |
| `/static/js/preeti-unicode.js` | Preeti → Unicode converter |
| `/static/js/itrans-unicode.js` | ITRANS → Devanagari converter |

## Data Layer

Documents, users, and sessions are stored in SQLite. `initDB` creates the schema on first run, `runMigrations` updates older databases, and `seedData` creates one sample document plus development admin/uploader accounts when the database is empty.

Document search uses SQLite FTS5 with the `unicode61` tokenizer, indexing title, Nepali title, description, and plain body text. Excerpts, store items, mission content, and similar-site content are still static/template-backed.

## Categories

Defined as a server-side slice and passed to templates:

```go
var docCategories = []string{"सबै", "किताब", "कागजात", "रेकर्ड", "पत्रिका", "अंश", "अन्य"}
```

These appear as filter tabs on the homepage and as options in the contribute form.
