package main

import (
	"embed"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strings"
)

//go:embed ui-translations.json
var localeFiles embed.FS

var uiCatalog = func() map[string]map[string]string {
	raw, err := localeFiles.ReadFile("ui-translations.json")
	if err != nil {
		panic(err)
	}
	var catalog map[string]map[string]string
	if err := json.Unmarshal(raw, &catalog); err != nil {
		panic(err)
	}
	return catalog
}()

func interfaceLanguage(r *http.Request) string {
	if c, err := r.Cookie("yogilib-language"); err == nil && c.Value == "ne" {
		return "ne"
	}
	return "en"
}

func translateUI(language, key string) string {
	if text := uiCatalog[key][language]; text != "" {
		return text
	}
	return key
}

func localizedTemplate(language string, files ...string) (*template.Template, error) {
	return template.New("base").Funcs(template.FuncMap{
		"t":          func(key string) string { return translateUI(language, key) },
		"uiLanguage": func() string { return language },
		"uiMessages": func() map[string]string {
			messages := make(map[string]string, len(uiCatalog))
			for key := range uiCatalog {
				messages[key] = translateUI(language, key)
			}
			return messages
		},
	}).ParseFiles(files...)
}

func languageHandler(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid language", http.StatusBadRequest)
		return
	}
	language := r.PostForm.Get("language")
	if language != "en" && language != "ne" {
		http.Error(w, "Invalid language", http.StatusBadRequest)
		return
	}
	next := r.PostForm.Get("return_to")
	parsed, err := url.Parse(next)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.ContainsAny(next, "\\\r\n") {
		next = "/"
	}
	http.SetCookie(w, &http.Cookie{Name: "yogilib-language", Value: language, Path: "/", MaxAge: 365 * 24 * 60 * 60, HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, next, http.StatusSeeOther)
}
