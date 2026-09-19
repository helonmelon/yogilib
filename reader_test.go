package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Opt-in PostgreSQL checks use connection-local temporary tables only.
func TestReaderDatabase(t *testing.T) {
	if os.Getenv("YOGILIB_READER_TEST") != "1" {
		t.Skip("set YOGILIB_READER_TEST=1 for temporary-table integration checks")
	}
	loadEnvFile(".env.local")
	testDB, err := sql.Open("pgx", os.Getenv("DATABASE_URL_UNPOOLED"))
	if err != nil {
		t.Fatal(err)
	}
	testDB.SetMaxOpenConns(1)
	defer testDB.Close()
	previous := db
	db = testDB
	defer func() { db = previous }()
	_, err = db.Exec(`CREATE TEMP TABLE documents (LIKE public.documents);
 CREATE TEMP TABLE users(id BIGINT,email TEXT,role TEXT);
 CREATE TEMP TABLE sessions(token TEXT,user_id BIGINT,expires_at TEXT);
 CREATE TEMP TABLE document_notes(id BIGINT GENERATED ALWAYS AS IDENTITY,document_id BIGINT,user_id BIGINT,quote TEXT,note TEXT);
 CREATE TEMP TABLE document_revisions(id BIGINT GENERATED ALWAYS AS IDENTITY,document_id BIGINT,snapshot JSONB,created_at TIMESTAMPTZ DEFAULT now());
 INSERT INTO documents(id,title,body_html,body_text,created_at) VALUES(9001,'Before','<p>Original</p>','Original','test');
 INSERT INTO users VALUES(9001,'test@example.invalid','admin'),(9002,'other@example.invalid','viewer');
 INSERT INTO sessions VALUES('reader-test',9001,'2099-01-01T00:00:00Z'),('other-test',9002,'2099-01-01T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body, token string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.SetPathValue("id", "9001")
		r.SetPathValue("note", "1")
		r.SetPathValue("revision", "1")
		r.AddCookie(&http.Cookie{Name: "session", Value: token})
		if strings.Contains(path, "edit") {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		w := httptest.NewRecorder()
		handler(w, r)
		return w
	}
	if w := request("POST", "/notes", `{"quote":"Original","note":"Private"}`, "reader-test", notesHandler); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("GET", "/notes", "", "other-test", notesHandler); strings.Contains(w.Body.String(), "Private") {
		t.Fatal("note leaked")
	}
	request("DELETE", "/notes/1", "", "other-test", notesHandler)
	if w := request("GET", "/notes", "", "reader-test", notesHandler); !strings.Contains(w.Body.String(), "Private") {
		t.Fatal("other user removed private note")
	}
	if w := request("POST", "/edit", "title=After&body_html=Changed", "reader-test", editPostHandler); w.Code != 303 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("POST", "/revisions/1", "", "reader-test", revisionsHandler); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	var title string
	db.QueryRow(`SELECT title FROM documents WHERE id=9001`).Scan(&title)
	if title != "Before" {
		t.Fatal("restore failed", title)
	}
	var count int
	db.QueryRow(`SELECT count(*) FROM document_revisions`).Scan(&count)
	if count != 2 {
		t.Fatal("restore must preserve previous version", count)
	}
	if w := request("GET", "/revisions", "", "other-test", requireRole("admin", revisionsHandler)); w.Code != 403 {
		t.Fatal("viewer accessed history", w.Code)
	}
}

func TestReaderOriginProtection(t *testing.T) {
	for _, tc := range []struct {
		origin, site string
		allowed      bool
	}{{"https://evil.example", "", false}, {"", "cross-site", false}, {"http://example.com", "same-origin", true}, {"", "", true}} {
		r := httptest.NewRequest("POST", "http://example.com/document/1/notes", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Sec-Fetch-Site", tc.site)
		if sameOrigin(httptest.NewRecorder(), r) != tc.allowed {
			t.Errorf("unexpected result for %+v", tc)
		}
	}
}

func TestReaderTemplateRoles(t *testing.T) {
	for _, role := range []string{"viewer", "admin"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/document/1", nil)
		render(w, r, "document", PageData{Doc: &Document{ID: "1", Title: "Test"}, User: &User{Role: role}})
		page := w.Body.String()
		if !strings.Contains(page, "reader-tools") || !strings.Contains(page, "save-note") {
			t.Fatal("reader controls missing")
		}
		if strings.Contains(page, `id="reader-history"`) != (role == "admin") {
			t.Fatal("revision history role visibility incorrect")
		}
	}
}

func TestDeleteControlVisibility(t *testing.T) {
	tests := []struct {
		name       string
		user       *User
		uploadedBy int
		wantDelete bool
	}{
		{name: "anonymous", wantDelete: false},
		{name: "owner", user: &User{ID: 8, Role: "uploader"}, uploadedBy: 8, wantDelete: true},
		{name: "other uploader", user: &User{ID: 9, Role: "uploader"}, uploadedBy: 8, wantDelete: false},
		{name: "admin", user: &User{ID: 1, Role: "admin"}, uploadedBy: 8, wantDelete: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/document/1", nil)
			render(w, r, "document", PageData{
				Doc:  &Document{ID: "1", Title: "Test", UploadedBy: tc.uploadedBy},
				User: tc.user,
			})
			hasDelete := strings.Contains(w.Body.String(), `action="/document/1/delete"`)
			if hasDelete != tc.wantDelete {
				t.Fatalf("delete control visibility = %v, want %v", hasDelete, tc.wantDelete)
			}
		})
	}
}
