package main

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLanguagePreferenceAndRedirect(t *testing.T) {
	for _, tc := range []struct {
		language, next, want string
		code                 int
	}{
		{"ne", "/dashboard?q=नेपाल&cat=किताब", "/dashboard?q=नेपाल&cat=किताब", 303},
		{"en", "/document/1#passage", "/document/1#passage", 303},
		{"ne", "https://example.com", "/", 303},
		{"ne", "//example.com", "/", 303},
		{"ne", "/\\example.com", "/", 303},
		{"fr", "/", "", 400},
	} {
		form := url.Values{"language": {tc.language}, "return_to": {tc.next}}
		r := httptest.NewRequest("POST", "https://yogilib.example/language", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		languageHandler(w, r)
		if w.Code != tc.code {
			t.Fatalf("%+v: status %d", tc, w.Code)
		}
		if tc.code != 303 {
			continue
		}
		location, _ := url.PathUnescape(w.Header().Get("Location"))
		if location != tc.want {
			t.Errorf("redirect: %q", w.Header().Get("Location"))
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].Path != "/" || cookies[0].MaxAge <= 0 {
			t.Fatal("invalid preference cookie")
		}
		next := httptest.NewRequest("GET", "/", nil)
		next.AddCookie(cookies[0])
		if interfaceLanguage(next) != tc.language {
			t.Fatal("preference not restored")
		}
	}
	r := httptest.NewRequest("POST", "/language", nil)
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	languageHandler(w, r)
	if w.Code != 403 || len(w.Result().Cookies()) != 0 {
		t.Fatal("cross-origin preference update accepted")
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "yogilib-language", Value: "invalid"})
	if interfaceLanguage(r) != "en" {
		t.Fatal("invalid language should fall back to English")
	}
}

func TestLocalizedPagesPreserveArchiveContent(t *testing.T) {
	for _, language := range []string{"en", "ne"} {
		for _, page := range []string{"index", "about", "works", "excerpts", "excerpt", "store", "similar", "mission", "login", "dashboard", "upload", "edit", "document"} {
			t.Run(language+"/"+page, func(t *testing.T) {
				r := httptest.NewRequest("GET", "/"+page+"?q=नेपाल", nil)
				r.AddCookie(&http.Cookie{Name: "yogilib-language", Value: language})
				w := httptest.NewRecorder()
				doc := Document{ID: "1", Title: "English original", TitleNP: "मूल नेपाली शीर्षक", Category: "किताब", Description: "मूल विवरण", BodyHTML: template.HTML("<p>नेपाली मूल पाठ — Find passage</p>"), Lang: "ne", FilePath: "/static/docs/test.pdf"}
				render(w, r, page, PageData{Title: "Dashboard", User: &User{Email: "reader@example.invalid", Role: "admin"}, Doc: &doc, Documents: []Document{doc}, Categories: docCategories, Exc: &Excerpt{Title: "मूल अंश", Body: template.HTML("<p>मूल पाठ</p>")}})
				body := w.Body.String()
				if w.Code != 200 || strings.Contains(body, "ZgotmplZ") || !strings.Contains(body, `lang="`+language+`"`) {
					t.Fatalf("failed rendering: %s", body)
				}
				if !strings.Contains(body, ">"+translateUI(language, "ड्यासबोर्ड")+"</a>") {
					t.Fatal("account navigation not translated")
				}
				if !strings.Contains(body, "reader@example.invalid") {
					t.Fatal("account identity lost")
				}
				if page == "document" && !strings.Contains(body, "<p>नेपाली मूल पाठ — Find passage</p>") {
					t.Fatal("archive body changed")
				}
				if page == "document" || page == "index" || page == "dashboard" {
					if !strings.Contains(body, "मूल नेपाली शीर्षक") || !strings.Contains(body, "English original") {
						t.Fatal("document titles changed")
					}
				}
				if page == "upload" && !strings.Contains(body, `value="किताब"`) {
					t.Fatal("stored category value changed")
				}
			})
		}
	}
}
