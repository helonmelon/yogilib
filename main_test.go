package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	if db != nil {
		db.Close()
		db = nil
	}
	if err := initDB(t.TempDir() + "/yogilib-test.db"); err != nil {
		t.Fatalf("initDB: %v", err)
	}
	t.Cleanup(func() {
		if db != nil {
			db.Close()
			db = nil
		}
	})
}

func TestRequireRoleRedirectsAnonymousUsers(t *testing.T) {
	setupTestDB(t)

	handler := requireRole("uploader", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("protected handler should not be called")
	})
	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/login" {
		t.Fatalf("Location = %q, want /login", got)
	}
}

func TestUploaderCanCreateDocument(t *testing.T) {
	setupTestDB(t)

	var userID int
	if err := db.QueryRow(`SELECT id FROM users WHERE email = ?`, "upload@yogilib.org").Scan(&userID); err != nil {
		t.Fatalf("find uploader: %v", err)
	}
	token, err := createSession(userID)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	fields := map[string]string{
		"title":          "Archive Test",
		"title_np":       "अभिलेख परीक्षण",
		"category":       "कागजात",
		"description":    "A document inserted by a handler test.",
		"body_html":      "<p>raresearchtoken appears here</p>",
		"doc_lang":       "ne",
		"script":         "devanagari",
		"orig_author":    "Test Author",
		"orig_author_np": "परीक्षण लेखक",
		"orig_year":      "1815",
		"orig_month":     "12",
		"orig_day":       "2",
	}
	for key, value := range fields {
		if err := form.WriteField(key, value); err != nil {
			t.Fatalf("write form field %s: %v", key, err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatalf("close form: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "session", Value: token})
	rec := httptest.NewRecorder()

	requireRole("uploader", uploadPostHandler)(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "/document/") {
		t.Fatalf("Location = %q, want /document/{id}", location)
	}
	id := strings.TrimPrefix(location, "/document/")
	doc := getDocumentByID(id)
	if doc == nil {
		t.Fatalf("document %s was not persisted", id)
	}
	if doc.Title != "Archive Test" || doc.UploadedBy != userID {
		t.Fatalf("persisted doc = %#v, want uploaded archive test doc", doc)
	}
	if doc.BodyText != "raresearchtoken appears here" {
		t.Fatalf("BodyText = %q, want stripped body text", doc.BodyText)
	}
}

func TestSearchMatchesDocumentBodyText(t *testing.T) {
	setupTestDB(t)

	_, err := db.Exec(`
		INSERT INTO documents (title, title_np, category, description, body_html, body_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "Body Search Test", "पूर्ण पाठ खोज", "अंश", "Search fixture", "<p>needlephrase</p>", "needlephrase", "11 Sep 2026")
	if err != nil {
		t.Fatalf("insert fixture: %v", err)
	}

	results := queryDocuments("needlephrase", "")
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1; results = %#v", len(results), results)
	}
	if results[0].Title != "Body Search Test" {
		t.Fatalf("result title = %q, want Body Search Test", results[0].Title)
	}
}
