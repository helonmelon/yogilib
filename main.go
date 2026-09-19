// yogilib — Go HTTP server
// Run:   go run .
// Build: go build -o yogilib .
package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

type Document struct {
	ID           string
	Title        string
	TitleNP      string
	Category     string
	Description  string
	BodyHTML     template.HTML
	BodyText     string
	Lang         string
	Script       string
	OrigAuthor   string
	OrigAuthorNP string
	OrigYear     string
	OrigMonth    string
	OrigDay      string
	FilePath     string
	UploadedBy   int
	CreatedAt    string
	CanEdit      bool
	CanDelete    bool
}

type Excerpt struct {
	Slug  string
	Title string
	Body  template.HTML
}

type StoreItem struct {
	ID          string
	Title       string
	TitleNP     string
	Description string
	ImageURL    string
	BuyURL      string
}

type User struct {
	ID    int
	Email string
	Role  string // "admin" | "uploader" | "viewer"
}

type PageData struct {
	Title      string
	Query      string
	Category   string
	Categories []string
	Flash      string
	Error      string
	Documents  []Document
	Doc        *Document
	Excerpts   []Excerpt
	Exc        *Excerpt
	StoreItems []StoreItem
	User       *User // nil when not logged in
	DevReload  bool
	ReturnTo   string
	PageClass  string
}

// ---------------------------------------------------------------------------
// Role hierarchy
// ---------------------------------------------------------------------------

var roleLevel = map[string]int{
	"viewer":   1,
	"uploader": 2,
	"admin":    3,
}

func hasRole(userRole, required string) bool {
	return roleLevel[userRole] >= roleLevel[required]
}

func canDeleteDocument(user *User, doc *Document) bool {
	if user == nil || doc == nil {
		return false
	}
	return user.Role == "admin" || (user.Role == "uploader" && doc.UploadedBy == user.ID)
}

// ---------------------------------------------------------------------------
// Database
// ---------------------------------------------------------------------------

var db *sql.DB

var docCategories = []string{"सबै", "किताब", "कागजात", "रेकर्ड", "पत्रिका", "अंश", "अन्य"}

func loadEnvFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key != "" && os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func initDB(databaseURL string) error {
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	var err error
	db, err = sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	schema := `
	CREATE TABLE IF NOT EXISTS document_revisions (
	 id BIGSERIAL PRIMARY KEY, document_id BIGINT NOT NULL,
	 snapshot JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE TABLE IF NOT EXISTS document_notes (
	 id BIGSERIAL PRIMARY KEY, document_id BIGINT NOT NULL,
	 user_id BIGINT NOT NULL, quote TEXT NOT NULL, note TEXT NOT NULL,
	 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	CREATE TABLE IF NOT EXISTS documents (
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

	CREATE TABLE IF NOT EXISTS users (
		id            BIGSERIAL PRIMARY KEY,
		email         TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role          TEXT NOT NULL DEFAULT 'viewer'
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token      TEXT PRIMARY KEY,
		user_id    BIGINT NOT NULL REFERENCES users(id),
		expires_at TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS documents_search_idx ON documents
	USING GIN (to_tsvector('simple',
		COALESCE(title, '') || ' ' ||
		COALESCE(title_np, '') || ' ' ||
		COALESCE(description, '') || ' ' ||
		COALESCE(body_text, '')
	));
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("schema: %w", err)
	}

	return seedData()
}

// ---------------------------------------------------------------------------
// Seed data
// ---------------------------------------------------------------------------

func seedData() error {
	var count int

	// Seed documents
	db.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	if count == 0 {
		bodyHTML := sugowleeBodyHTML()
		_, err := db.Exec(`
			INSERT INTO documents (
				title, title_np, category, description,
				body_html, body_text, lang, script,
				orig_author, orig_author_np,
				orig_year, orig_month, orig_day, created_at
			) VALUES (
				$1, $2, $3, $4,
				$5, $6, $7, $8,
				$9, $10,
				$11, $12, $13, $14
			)
		`,
			"Treaty of Sugowlee / Treaty of Sugauli",
			"सुगौली सन्धि",
			"कागजात",
			"Full bilingual document text for the 1815 Treaty of Sugauli between the East India Company and the Kingdom of Nepal.",
			bodyHTML,
			stripHTML(bodyHTML),
			"ne-en",
			"mixed",
			"East India Company and the Kingdom of Nepal",
			"इस्ट इन्डिया कम्पनी र नेपाल अधिराज्य",
			"1815",
			"12",
			"2",
			time.Now().AddDate(0, 0, -3).Format("2 Jan 2006"),
		)
		if err != nil {
			return err
		}
	}

	// Seed users
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count == 0 {
		seed := []struct{ email, password, role string }{
			{"admin@yogilib.org", "admin123", "admin"},
			{"upload@yogilib.org", "upload123", "uploader"},
		}
		for _, u := range seed {
			hash, err := bcrypt.GenerateFromPassword([]byte(u.password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if _, err := db.Exec(
				`INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3)`,
				u.email, string(hash), u.role,
			); err != nil {
				return err
			}
		}
		log.Println("seed users: admin@yogilib.org/admin123  upload@yogilib.org/upload123")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var reHTMLTags = regexp.MustCompile(`<[^>]+>`)

// stripHTML removes HTML tags, leaving plain text suitable for FTS indexing.
func stripHTML(s string) string {
	plain := reHTMLTags.ReplaceAllString(s, " ")
	// Collapse whitespace
	return strings.Join(strings.Fields(plain), " ")
}

func sugowleeBodyHTML() string {
	return `<section>
  <h2>सुगौली सन्धि / Treaty of Sugauli</h2>
  <p><strong>Date:</strong> 2 December 1815; ratified 4 March 1816.</p>
  <p><strong>Parties:</strong> The Honourable East India Company and the King of Nepal.</p>
  <p><strong>Representatives:</strong> Lieutenant-Colonel Paris Bradshaw for the East India Company; Raj Guru Gajraj Mishra and Chandra Shekhar Upadhyay for Nepal.</p>

  <h3>नेपाली रूपान्तरण</h3>
  <p><strong>धारा १</strong> माननीय इस्ट इन्डिया कम्पनी र नेपालका राजाबीच चिरस्थायी शान्ति र मैत्री रहनेछ।</p>
  <p><strong>धारा २</strong> नेपालका राजाले युद्धअघि दुई राज्यबीच विवादमा रहेका सबै भूमिमाथिको दाबी त्याग्नेछन्, र ती भूमिमाथि माननीय कम्पनीको सार्वभौमिक अधिकार स्वीकार गर्नेछन्।</p>
  <p><strong>धारा ३</strong> नेपालका राजाले माननीय इस्ट इन्डिया कम्पनीलाई काली र राप्ती नदीबीचका तल्लो भूभाग, राप्ती र गण्डकीबीच बुटवल खासबाहेकका तल्लो भूभाग, गण्डकी र कोशीबीच ब्रिटिश अधिकार स्थापित भएका वा हुँदै गरेका तल्लो भूभाग, मेची र टिस्टाबीचका तल्लो भूभाग, र मेचीपूर्वका पहाडी भूभागहरू सदाका लागि हस्तान्तरण गर्छन्। यी भूभागहरू गोर्खाली सेनाले यस मितिबाट चालिस दिनभित्र खाली गर्नुपर्नेछ।</p>
  <p><strong>धारा ४</strong> हस्तान्तरण गरिएका भूमिले क्षति पुगेका नेपाल राज्यका प्रमुख र भारदारहरूलाई क्षतिपूर्ति दिन ब्रिटिश सरकारले नेपालका राजाले छनोट गर्ने प्रमुखहरूलाई, राजाले तोक्ने अनुपातमा, वार्षिक जम्मा दुई लाख रुपैयाँ पेन्सन दिने सहमति गर्छ।</p>
  <p><strong>धारा ५</strong> नेपालका राजाले आफैं, आफ्ना उत्तराधिकारी र वंशजहरूको तर्फबाट काली नदीको पश्चिमपट्टिका देशहरूमाथिको सबै दाबी र सम्बन्ध त्याग्छन्।</p>
  <p><strong>धारा ६</strong> नेपालका राजाले सिक्किमका राजालाई उनका भूभागहरूको अधिकारमा कहिल्यै हैरान वा बाधा नगर्ने प्रतिज्ञा गर्छन्, र कुनै मतभेद उठेमा ब्रिटिश सरकारको मध्यस्थता स्वीकार गर्नेछन्।</p>
  <p><strong>धारा ७</strong> नेपालका राजाले ब्रिटिश सरकारको स्वीकृतिविना कुनै ब्रिटिश प्रजा, वा कुनै युरोपेली अथवा अमेरिकी राज्यको प्रजालाई आफ्नो सेवामा नलिने वा सेवामा नराख्ने प्रतिज्ञा गर्छन्।</p>
  <p><strong>धारा ८</strong> यस सन्धिबाट स्थापित मैत्री र शान्तिको सम्बन्ध सुरक्षित र सुदृढ पार्न, प्रत्येक राज्यबाट मान्यताप्राप्त मन्त्री अर्को राज्यको दरबारमा बस्ने सहमति गरिन्छ।</p>
  <p><strong>धारा ९</strong> नौ धाराबाट बनेको यो सन्धि नेपालका राजाले यस मितिबाट पन्ध्र दिनभित्र अनुमोदन गर्नेछन्, र अनुमोदन लेफ्टिनेन्ट-कर्नेल ब्राडशालाई बुझाइनेछ।</p>

  <h3>English Text</h3>
  <p><strong>Article I</strong> There shall be perpetual peace and friendship between the Honourable East India Company and the King of Nepal.</p>
  <p><strong>Article II</strong> The Rajah of Nepal renounces all claim to the lands which were the subject of discussion between the two States before the war, and acknowledges the right of the Honourable Company to the sovereignty of those lands.</p>
  <p><strong>Article III</strong> The Rajah of Nepal hereby cedes to the Honourable the East India Company in perpetuity the lowlands between the Rivers Kali and Rapti; the lowlands, except Bootwul Khass, between the Rapti and Gunduck; the lowlands between the Gunduck and Coosah where British authority had been introduced or was being introduced; the lowlands between the Mitchee and Teestah; and hill territories eastward of the River Mitchee, including Nagree and the Pass of Nagarcote. The Gurkha troops shall evacuate the territory within forty days.</p>
  <p><strong>Article IV</strong> To indemnify the Chiefs and Barahdars of Nepal whose interests suffer by the cession, the British Government agrees to settle pensions totaling two lakhs of rupees per annum on chiefs selected by the Rajah of Nepal.</p>
  <p><strong>Article V</strong> The Rajah of Nepal renounces for himself, his heirs, and successors all claim to or connection with the countries lying to the west of the River Kali.</p>
  <p><strong>Article VI</strong> The Rajah of Nepal agrees not to disturb the Rajah of Sikkim in his territories, and to refer disputes with Sikkim to the arbitration of the British Government.</p>
  <p><strong>Article VII</strong> The Rajah of Nepal agrees not to take or retain in his service any British subject, or any subject of a European or American state, without the consent of the British Government.</p>
  <p><strong>Article VIII</strong> To secure and improve relations of amity and peace, accredited ministers from each state shall reside at the court of the other.</p>
  <p><strong>Article IX</strong> This treaty, consisting of nine articles, shall be ratified by the Rajah of Nepal within fifteen days, and the Governor-General's ratification shall be obtained and delivered as soon as practicable.</p>

  <h3>Source Notes</h3>
  <p>English text follows the public-domain Wikisource transcription of the Treaty of Sugauli, with a Nepali rendering prepared from the same clauses and cross-checked against Nepali references.</p>
  <p><a href="https://en.wikisource.org/wiki/Treaty_of_Sugauli">English source: Wikisource</a></p>
  <p><a href="https://ne.wikipedia.org/wiki/%E0%A4%B8%E0%A5%81%E0%A4%97%E0%A5%8C%E0%A4%B2%E0%A5%80_%E0%A4%B8%E0%A4%A8%E0%A5%8D%E0%A4%A7%E0%A4%BF">Nepali background: Wikipedia</a></p>
</section>`
}

// ---------------------------------------------------------------------------
// DB query functions
// ---------------------------------------------------------------------------

const docSelectCols = `
	d.id,
	d.title,
	COALESCE(d.title_np,''),
	COALESCE(d.category,''),
	COALESCE(d.description,''),
	COALESCE(d.body_html,''),
	COALESCE(d.body_text,''),
	COALESCE(d.lang,''),
	COALESCE(d.script,''),
	COALESCE(d.orig_author,''),
	COALESCE(d.orig_author_np,''),
	COALESCE(d.orig_year,''),
	COALESCE(d.orig_month,''),
	COALESCE(d.orig_day,''),
	COALESCE(d.file_path,''),
	COALESCE(d.uploaded_by,0),
	d.created_at
`

func scanDoc(rows interface{ Scan(...any) error }) (Document, error) {
	var d Document
	var id int
	var bodyHTML string
	err := rows.Scan(
		&id, &d.Title, &d.TitleNP, &d.Category, &d.Description,
		&bodyHTML, &d.BodyText,
		&d.Lang, &d.Script,
		&d.OrigAuthor, &d.OrigAuthorNP,
		&d.OrigYear, &d.OrigMonth, &d.OrigDay,
		&d.FilePath, &d.UploadedBy, &d.CreatedAt,
	)
	d.ID = strconv.Itoa(id)
	d.BodyHTML = template.HTML(bodyHTML) // trusted: uploaded by authenticated users only
	return d, err
}

func queryDocuments(q, cat string) []Document {
	var rows *sql.Rows
	var err error

	catFilter := cat != "" && cat != "सबै"

	if q != "" {
		ftsQuery := strings.TrimSpace(q)
		likeQuery := "%" + ftsQuery + "%"
		if catFilter {
			rows, err = db.Query(`
				SELECT `+docSelectCols+`
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
				  AND d.category = $3
				ORDER BY d.created_at DESC
			`, ftsQuery, likeQuery, cat)
		} else {
			rows, err = db.Query(`
				SELECT `+docSelectCols+`
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
				ORDER BY d.created_at DESC
			`, ftsQuery, likeQuery)
		}
	} else if catFilter {
		rows, err = db.Query(`
			SELECT `+docSelectCols+`
			FROM documents d
			WHERE d.category = $1
			ORDER BY d.created_at DESC
	`, cat)
	} else {
		rows, err = db.Query(`
			SELECT ` + docSelectCols + `
			FROM documents d
			ORDER BY d.created_at DESC
		`)
	}

	if err != nil {
		log.Println("queryDocuments:", err)
		return nil
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			log.Println("scan:", err)
			continue
		}
		docs = append(docs, d)
	}
	return docs
}

func getDocumentByID(id string) *Document {
	row := db.QueryRow(`
		SELECT `+docSelectCols+`
		FROM documents d
		WHERE d.id = $1
	`, id)
	d, err := scanDoc(row)
	if err != nil {
		return nil
	}
	return &d
}

func getExcerpts() []Excerpt {
	return []Excerpt{
		{Slug: "sugowlee", Title: "Treaty of Sugowlee, Dec. 2, 1815"},
	}
}

func getExcerptBySlug(slug string) *Excerpt {
	bodies := map[string]template.HTML{
		"sugowlee": `<p>Articles of Treaty concluded between the Honourable East India Company
		and the Rajah of Nepaul, signed by Lieutenant-Colonel Paris Bradshaw,
		Acting Political Agent at Nepaul, on the part of the Honourable East India Company,
		and by Gaj Raj Misser and Chunder Seekur Oophadaya, authorised Sirdars of the Rajah
		of Nepaul, on the part of His Highness the Rajah, on the 2nd day of December 1815.</p>`,
	}
	b, ok := bodies[slug]
	if !ok {
		return nil
	}
	return &Excerpt{Slug: slug, Title: "Treaty of Sugowlee, Dec. 2, 1815", Body: b}
}

// ---------------------------------------------------------------------------
// Session management
// ---------------------------------------------------------------------------

func newToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func createSession(userID int) (string, error) {
	token := newToken()
	expires := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		token, userID, expires)
	return token, err
}

func deleteSession(token string) {
	db.Exec(`DELETE FROM sessions WHERE token = $1`, token)
}

func sessionUser(r *http.Request) *User {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	var u User
	var expiresAt string
	err = db.QueryRow(`
		SELECT u.id, u.email, u.role, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = $1
	`, cookie.Value).Scan(&u.ID, &u.Email, &u.Role, &expiresAt)
	if err != nil {
		return nil
	}
	exp, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().After(exp) {
		deleteSession(cookie.Value)
		return nil
	}
	return &u
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   30 * 24 * 60 * 60,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

// ---------------------------------------------------------------------------
// Auth middleware
// ---------------------------------------------------------------------------

func requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := sessionUser(r)
		if u == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !hasRole(u.Role, role) {
			http.Error(w, translateUI(interfaceLanguage(r), "Forbidden"), http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// ---------------------------------------------------------------------------
// Template rendering
// ---------------------------------------------------------------------------

func render(w http.ResponseWriter, r *http.Request, page string, data PageData) {
	data.ReturnTo = r.URL.RequestURI()
	data.PageClass = "page-" + page
	data.DevReload = os.Getenv("DEV_RELOAD") == "1"
	if data.User == nil {
		data.User = sessionUser(r)
	}
	if data.Doc != nil {
		data.Doc.CanEdit = data.User != nil && data.User.Role == "admin"
		data.Doc.CanDelete = canDeleteDocument(data.User, data.Doc)
	}
	for i := range data.Documents {
		data.Documents[i].CanEdit = data.User != nil && data.User.Role == "admin"
		data.Documents[i].CanDelete = canDeleteDocument(data.User, &data.Documents[i])
	}
	t, err := localizedTemplate(interfaceLanguage(r),
		"templates/base.html",
		"templates/"+page+".html",
	)
	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Println("render:", err)
	}
}

func devVersionHandler(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("DEV_RELOAD") != "1" {
		http.NotFound(w, r)
		return
	}
	paths := []string{"templates/base.html", "templates/document.html", "static/css/style.css", "static/css/reader.css", "static/js/reader.js"}
	var newest int64
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.ModTime().UnixNano() > newest {
			newest = info.ModTime().UnixNano()
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, "%d", newest)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func indexHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	cat := r.URL.Query().Get("cat")
	render(w, r, "index", PageData{
		Title:      "Home",
		Query:      q,
		Category:   cat,
		Categories: docCategories,
		Documents:  queryDocuments(q, cat),
	})
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "about", PageData{Title: "About Yogi Narharinath — योगीबारे"})
}

func worksHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "works", PageData{Title: "Works — ग्रन्थावली"})
}

func excerptsHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "excerpts", PageData{
		Title:    "Excerpts — अंशहरू",
		Excerpts: getExcerpts(),
	})
}

func excerptHandler(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	exc := getExcerptBySlug(slug)
	if exc == nil {
		http.NotFound(w, r)
		return
	}
	render(w, r, "excerpt", PageData{Title: exc.Title, Exc: exc})
}

func documentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc := getDocumentByID(id)
	if doc == nil {
		http.NotFound(w, r)
		return
	}
	render(w, r, "document", PageData{Title: doc.Title, Doc: doc})
}

func editGetHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc := getDocumentByID(id)
	if doc == nil {
		http.NotFound(w, r)
		return
	}
	render(w, r, "edit", PageData{Title: "Edit: " + doc.Title, Doc: doc})
}

func editPostHandler(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Bad request"), http.StatusBadRequest)
		return
	}
	bodyHTML := r.FormValue("body_html")
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Database error"), 500)
		return
	}
	defer tx.Rollback()
	if err = snapshotDocument(tx, id); err != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Document unavailable"), 404)
		return
	}
	_, err = tx.Exec(`
		UPDATE documents SET
			title          = $1,
			title_np       = $2,
			category       = $3,
			description    = $4,
			lang           = $5,
			script         = $6,
			orig_author    = $7,
			orig_author_np = $8,
			orig_year      = $9,
			orig_month     = $10,
			orig_day       = $11,
			body_html      = $12,
			body_text      = $13
		WHERE id = $14
	`,
		r.FormValue("title"),
		r.FormValue("title_np"),
		r.FormValue("category"),
		r.FormValue("description"),
		r.FormValue("lang"),
		r.FormValue("script"),
		r.FormValue("orig_author"),
		r.FormValue("orig_author_np"),
		r.FormValue("orig_year"),
		r.FormValue("orig_month"),
		r.FormValue("orig_day"),
		bodyHTML,
		stripHTML(bodyHTML),
		id,
	)
	if err != nil {
		log.Println("editPost:", err)
		http.Error(w, translateUI(interfaceLanguage(r), "Database error"), http.StatusInternalServerError)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Database error"), 500)
		return
	}
	http.Redirect(w, r, "/document/"+id, http.StatusSeeOther)
}

func uploadGetHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "upload", PageData{Title: "Contribute — योगदान"})
}

func uploadPostHandler(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 50 << 20 // 50 MB
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Bad request"), http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		render(w, r, "upload", PageData{Title: "Contribute — योगदान", Error: "Title is required."})
		return
	}

	// Handle optional file attachment
	filePath := ""
	file, header, fileErr := r.FormFile("file")
	if fileErr == nil {
		defer file.Close()

		// Ensure upload directory exists
		if err := os.MkdirAll("static/docs", 0755); err != nil {
			log.Println("mkdir static/docs:", err)
			http.Error(w, translateUI(interfaceLanguage(r), "Server error"), http.StatusInternalServerError)
			return
		}

		// Build a unique filename: timestamp + original name
		ext := filepath.Ext(header.Filename)
		safeName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
		dst := filepath.Join("static", "docs", safeName)

		out, err := os.Create(dst)
		if err != nil {
			log.Println("create file:", err)
			http.Error(w, translateUI(interfaceLanguage(r), "Server error"), http.StatusInternalServerError)
			return
		}
		defer out.Close()

		if _, err := io.Copy(out, file); err != nil {
			log.Println("copy file:", err)
			http.Error(w, translateUI(interfaceLanguage(r), "Server error"), http.StatusInternalServerError)
			return
		}
		filePath = "/static/docs/" + safeName
	}

	// Body content from Quill
	bodyHTML := r.FormValue("body_html")
	bodyText := stripHTML(bodyHTML)

	// Uploader attribution
	uploadedBy := 0
	if u := sessionUser(r); u != nil {
		uploadedBy = u.ID
	}

	var id int64
	err := db.QueryRow(`
		INSERT INTO documents (
			title, title_np, category, description,
			body_html, body_text,
			lang, script,
			orig_author, orig_author_np,
			orig_year, orig_month, orig_day,
			file_path, uploaded_by, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8,
			$9, $10,
			$11, $12, $13,
			$14, $15, $16
		)
		RETURNING id
	`,
		title,
		r.FormValue("title_np"),
		r.FormValue("category"),
		r.FormValue("description"),
		bodyHTML,
		bodyText,
		r.FormValue("doc_lang"),
		r.FormValue("script"),
		r.FormValue("orig_author"),
		r.FormValue("orig_author_np"),
		r.FormValue("orig_year"),
		r.FormValue("orig_month"),
		r.FormValue("orig_day"),
		filePath,
		uploadedBy,
		time.Now().Format("2 Jan 2006"),
	).Scan(&id)
	if err != nil {
		log.Println("uploadPost:", err)
		http.Error(w, translateUI(interfaceLanguage(r), "Database error"), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/document/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func documentDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(w, r) {
		return
	}
	u := sessionUser(r)
	doc := getDocumentByID(r.PathValue("id"))
	if doc == nil {
		http.NotFound(w, r)
		return
	}
	if !canDeleteDocument(u, doc) {
		http.Error(w, translateUI(interfaceLanguage(r), "You can only delete documents you uploaded."), http.StatusForbidden)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Database error"), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM document_notes WHERE document_id=$1`, doc.ID); err == nil {
		_, err = tx.Exec(`DELETE FROM document_revisions WHERE document_id=$1`, doc.ID)
	}
	if err == nil {
		var result sql.Result
		result, err = tx.Exec(`DELETE FROM documents WHERE id=$1`, doc.ID)
		if err == nil {
			if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
				err = fmt.Errorf("delete document affected %d rows: %v", affected, rowsErr)
			}
		}
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		log.Println("documentDelete:", err)
		http.Error(w, translateUI(interfaceLanguage(r), "Could not delete document"), http.StatusInternalServerError)
		return
	}

	// Attachments are only removed when they resolve inside the local upload directory.
	if doc.FilePath != "" {
		uploadRoot := filepath.Clean(filepath.Join("static", "docs"))
		localPath := filepath.Clean(strings.TrimPrefix(doc.FilePath, "/"))
		if relative, relErr := filepath.Rel(uploadRoot, localPath); relErr == nil && relative != "." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != ".." {
			if removeErr := os.Remove(localPath); removeErr != nil && !os.IsNotExist(removeErr) {
				log.Println("remove deleted document attachment:", removeErr)
			}
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func missionHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "mission", PageData{Title: "Mission — उद्देश्य"})
}

func similarHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "similar", PageData{Title: "Similar Sites — अरु साइट"})
}

func storeHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "store", PageData{Title: "Store — पसल"})
}

func loginGetHandler(w http.ResponseWriter, r *http.Request) {
	if sessionUser(r) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	render(w, r, "login", PageData{Title: "Login"})
}

func loginPostHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Bad request"), http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	var u User
	var hash string
	err := db.QueryRow(
		`SELECT id, email, password_hash, role FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &hash, &u.Role)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		render(w, r, "login", PageData{Title: "Login", Error: "Invalid email or password."})
		return
	}

	token, err := createSession(u.ID)
	if err != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Session error"), http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, token)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		deleteSession(cookie.Value)
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	cat := r.URL.Query().Get("cat")
	render(w, r, "dashboard", PageData{
		Title:      "Dashboard",
		Query:      q,
		Category:   cat,
		Categories: docCategories,
		Documents:  queryDocuments(q, cat),
	})
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func sameOrigin(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, translateUI(interfaceLanguage(r), "Forbidden"), 403)
		return false
	}
	origin := r.Header.Get("Origin")
	if origin != "" && origin != "http://"+r.Host && origin != "https://"+r.Host {
		http.Error(w, translateUI(interfaceLanguage(r), "Forbidden"), 403)
		return false
	}
	return true
}

func snapshotDocument(tx *sql.Tx, id string) error {
	var snapshot []byte
	if err := tx.QueryRow(`SELECT to_jsonb(d) FROM documents d WHERE id=$1 FOR UPDATE`, id).Scan(&snapshot); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO document_revisions(document_id,snapshot) VALUES($1,$2::jsonb)`, id, string(snapshot))
	return err
}

func notesHandler(w http.ResponseWriter, r *http.Request) {
	u := sessionUser(r)
	id := r.PathValue("id")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" && !sameOrigin(w, r) {
		return
	}
	if getDocumentByID(id) == nil {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case "POST":
		var n struct {
			Quote string `json:"quote"`
			Note  string `json:"note"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 20000)
		if json.NewDecoder(r.Body).Decode(&n) != nil || strings.TrimSpace(n.Quote) == "" || len(n.Quote) > 10000 || len(n.Note) > 5000 {
			http.Error(w, translateUI(interfaceLanguage(r), "Select a shorter passage and note"), 400)
			return
		}
		_, err := db.Exec(`INSERT INTO document_notes(document_id,user_id,quote,note) VALUES($1,$2,$3,$4)`, id, u.ID, n.Quote, n.Note)
		if err != nil {
			http.Error(w, translateUI(interfaceLanguage(r), "Could not save note"), 500)
			return
		}
		w.WriteHeader(201)
	case "DELETE":
		_, err := db.Exec(`DELETE FROM document_notes WHERE id=$1 AND document_id=$2 AND user_id=$3`, r.PathValue("note"), id, u.ID)
		if err != nil {
			http.Error(w, translateUI(interfaceLanguage(r), "Could not remove note"), 500)
			return
		}
		w.WriteHeader(204)
	default:
		rows, err := db.Query(`SELECT id,quote,note FROM document_notes WHERE document_id=$1 AND user_id=$2 ORDER BY id`, id, u.ID)
		if err != nil {
			http.Error(w, translateUI(interfaceLanguage(r), "Could not load notes"), 500)
			return
		}
		defer rows.Close()
		type note struct {
			ID    int    `json:"id"`
			Quote string `json:"quote"`
			Note  string `json:"note"`
		}
		result := []note{}
		for rows.Next() {
			var n note
			if rows.Scan(&n.ID, &n.Quote, &n.Note) != nil {
				http.Error(w, translateUI(interfaceLanguage(r), "Database error"), 500)
				return
			}
			result = append(result, n)
		}
		if rows.Err() != nil {
			http.Error(w, translateUI(interfaceLanguage(r), "Database error"), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func revisionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id := r.PathValue("id")
	if r.Method == "POST" {
		if !sameOrigin(w, r) {
			return
		}
		tx, err := db.Begin()
		if err != nil {
			http.Error(w, translateUI(interfaceLanguage(r), "Database error"), 500)
			return
		}
		defer tx.Rollback()
		if snapshotDocument(tx, id) != nil {
			http.NotFound(w, r)
			return
		}
		result, err := tx.Exec(`UPDATE documents d SET title=s.title,title_np=s.title_np,category=s.category,description=s.description,body_html=s.body_html,body_text=s.body_text,lang=s.lang,script=s.script,orig_author=s.orig_author,orig_author_np=s.orig_author_np,orig_year=s.orig_year,orig_month=s.orig_month,orig_day=s.orig_day FROM document_revisions r CROSS JOIN LATERAL jsonb_populate_record(NULL::documents,r.snapshot) s WHERE r.id=$1 AND r.document_id=$2 AND d.id=r.document_id`, r.PathValue("revision"), id)
		if err != nil {
			http.Error(w, translateUI(interfaceLanguage(r), "Could not restore revision"), 500)
			return
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			http.NotFound(w, r)
			return
		}
		if tx.Commit() != nil {
			http.Error(w, translateUI(interfaceLanguage(r), "Could not restore revision"), 500)
			return
		}
		w.WriteHeader(204)
		return
	}
	rows, err := db.Query(`SELECT id,created_at::text,snapshot->>'title',snapshot->>'body_text' FROM document_revisions WHERE document_id=$1 ORDER BY id DESC`, id)
	if err != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Could not load history"), 500)
		return
	}
	defer rows.Close()
	type revision struct {
		ID    int    `json:"id"`
		Date  string `json:"date"`
		Title string `json:"title"`
		Text  string `json:"text"`
	}
	result := []revision{}
	for rows.Next() {
		var v revision
		if rows.Scan(&v.ID, &v.Date, &v.Title, &v.Text) != nil {
			http.Error(w, translateUI(interfaceLanguage(r), "Database error"), 500)
			return
		}
		result = append(result, v)
	}
	if rows.Err() != nil {
		http.Error(w, translateUI(interfaceLanguage(r), "Database error"), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func main() {
	loadEnvFile(".env.local")
	if err := initDB(os.Getenv("DATABASE_URL")); err != nil {
		log.Fatal("db:", err)
	}
	defer db.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /language", languageHandler)

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Public — no login needed
	mux.HandleFunc("GET /{$}", indexHandler)
	mux.HandleFunc("GET /about", aboutHandler)
	mux.HandleFunc("GET /works", worksHandler)
	mux.HandleFunc("GET /excerpts", excerptsHandler)
	mux.HandleFunc("GET /excerpts/{slug}", excerptHandler)
	mux.HandleFunc("GET /document/{id}", documentHandler)
	mux.HandleFunc("GET /__dev/version", devVersionHandler)
	mux.HandleFunc("GET /document/{id}/notes", requireRole("viewer", notesHandler))
	mux.HandleFunc("POST /document/{id}/notes", requireRole("viewer", notesHandler))
	mux.HandleFunc("DELETE /document/{id}/notes/{note}", requireRole("viewer", notesHandler))
	mux.HandleFunc("GET /document/{id}/revisions", requireRole("admin", revisionsHandler))
	mux.HandleFunc("POST /document/{id}/revisions/{revision}", requireRole("admin", revisionsHandler))
	mux.HandleFunc("GET /mission", missionHandler)
	mux.HandleFunc("GET /similar", similarHandler)
	mux.HandleFunc("GET /store", storeHandler)
	mux.HandleFunc("GET /login", loginGetHandler)
	mux.HandleFunc("POST /login", loginPostHandler)
	mux.HandleFunc("POST /logout", logoutHandler)

	// Uploader+ only
	mux.HandleFunc("GET /upload", requireRole("uploader", uploadGetHandler))
	mux.HandleFunc("POST /upload", requireRole("uploader", uploadPostHandler))
	mux.HandleFunc("POST /document/{id}/delete", requireRole("uploader", documentDeleteHandler))

	// Admin only
	mux.HandleFunc("GET /dashboard", requireRole("admin", dashboardHandler))
	mux.HandleFunc("GET /document/{id}/edit", requireRole("admin", editGetHandler))
	mux.HandleFunc("POST /document/{id}/edit", requireRole("admin", editPostHandler))

	log.Printf("yogilib → http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
