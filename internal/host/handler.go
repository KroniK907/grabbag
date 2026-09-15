package host

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"net"
	"net/http"
	"strings"

	"github.com/KroniK907/hackbox/internal/store"
)

const adminCookieName = "hackbox_admin"

//go:embed templates/*.html
var templateFiles embed.FS

var pageTemplates = template.Must(template.ParseFS(templateFiles, "templates/*.html"))

// NewHandler returns the host routes wrapped in Go's cross-origin protection.
func NewHandler(db *store.DB) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /setup", getSetup(db))
	mux.HandleFunc("POST /setup", finishSetup(db))
	mux.HandleFunc("GET /{$}", getStub(db, "Hackbox phone"))
	mux.HandleFunc("GET /board", getStub(db, "Hackbox board"))
	mux.HandleFunc("GET /settings", getStub(db, "Hackbox settings"))
	return http.NewCrossOriginProtection().Handler(mux)
}

func getSetup(db *store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hasHash, err := db.HasAdminHash(r.Context())
		if err != nil {
			http.Error(w, "Could not read setup state.", http.StatusInternalServerError)
			return
		}
		if hasHash {
			http.Redirect(w, r, "/board", http.StatusSeeOther)
			return
		}
		writeSetupPage(w, "", http.StatusOK)
	}
}

func finishSetup(db *store.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hasHash, err := db.HasAdminHash(r.Context())
		if err != nil {
			http.Error(w, "Could not read setup state.", http.StatusInternalServerError)
			return
		}
		if hasHash {
			http.Error(w, "Setup is already finished.", http.StatusConflict)
			return
		}
		if err := r.ParseForm(); err != nil {
			writeSetupPage(w, "Could not read the form.", http.StatusBadRequest)
			return
		}
		password := r.PostFormValue("password")
		switch {
		case password != r.PostFormValue("confirm"):
			writeSetupPage(w, "Passwords do not match.", http.StatusBadRequest)
			return
		case len(password) < 8:
			writeSetupPage(w, "Password must be at least 8 characters.", http.StatusBadRequest)
			return
		}

		hash, err := hashPassword(password)
		if err != nil {
			http.Error(w, "Could not finish setup.", http.StatusInternalServerError)
			return
		}
		sessionID, err := newSessionID()
		if err != nil {
			http.Error(w, "Could not finish setup.", http.StatusInternalServerError)
			return
		}
		if err := db.FinishSetup(r.Context(), hash, sessionID); err != nil {
			if errors.Is(err, store.ErrAdminHashExists) {
				http.Error(w, "Setup is already finished.", http.StatusConflict)
				return
			}
			http.Error(w, "Could not finish setup.", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     adminCookieName,
			Value:    sessionID,
			Path:     "/",
			HttpOnly: true,
			Secure:   secureAdminCookie(r),
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/board", http.StatusSeeOther)
	}
}

func getStub(db *store.DB, title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hasHash, err := db.HasAdminHash(r.Context())
		if err != nil {
			http.Error(w, "Could not read setup state.", http.StatusInternalServerError)
			return
		}
		if !hasHash {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		renderPage(w, "stub.html", struct{ Title string }{Title: title}, http.StatusOK)
	}
}

func writeSetupPage(w http.ResponseWriter, message string, status int) {
	renderPage(w, "setup.html", struct{ Error string }{Error: message}, status)
}

func renderPage(w http.ResponseWriter, name string, data any, status int) {
	var body bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&body, name, data); err != nil {
		http.Error(w, "Could not render the page.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = body.WriteTo(w)
}

func secureAdminCookie(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host := r.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.Trim(host, "[]")
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1"
}
