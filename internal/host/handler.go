package host

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"net"
	"net/http"
	"strings"

	"github.com/KroniK907/hackbox/internal/lobby"
	"github.com/KroniK907/hackbox/internal/platform/hub"
	"github.com/KroniK907/hackbox/internal/store"
	"github.com/KroniK907/hackbox/internal/ui"
)

const adminCookieName = "hackbox_admin"

//go:embed templates/*.html
var templateFiles embed.FS

var pageTemplates = template.Must(template.ParseFS(templateFiles, "templates/*.html"))

// NewHandler returns the host routes wrapped in Go's cross-origin protection.
// lanJoinURL is the fallback join address shown on /board when the request
// host is loopback. It may be empty when no usable LAN IPv4 exists. A public
// hostname such as a Cloudflare tunnel replaces that fallback.
func NewHandler(db *store.DB, lanJoinURL string) (http.Handler, error) {
	events := hub.New()
	room, err := lobby.New(db, lobby.Config{
		AdminCookieName: adminCookieName,
		Events:          events,
		PasswordMatches: passwordMatches,
		SecureCookie:    secureAdminCookie,
	})
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", ui.StaticHandler()))
	mux.HandleFunc("GET /setup", getSetup(db))
	mux.HandleFunc("POST /setup", finishSetup(db))
	mux.Handle("GET /{$}", requireSetup(db, http.HandlerFunc(room.Phone)))
	mux.Handle("GET /board", requireSetup(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		room.Board(w, r, joinURLForRequest(r, lanJoinURL))
	})))
	mux.HandleFunc("GET /settings", getStub(db, "Hackbox settings", "", false))
	lobbyWrites := http.NewServeMux()
	room.Register(lobbyWrites)
	mux.Handle("/lobby/", requireSetup(db, lobbyWrites))
	return http.NewCrossOriginProtection().Handler(mux), nil
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
			MaxAge:   30 * 24 * 60 * 60,
			HttpOnly: true,
			Secure:   secureAdminCookie(r),
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/board", http.StatusSeeOther)
	}
}

func getStub(db *store.DB, title, lanJoinURL string, advertiseJoin bool) http.HandlerFunc {
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
		joinURL := ""
		if advertiseJoin {
			joinURL = joinURLForRequest(r, lanJoinURL)
		}
		renderPage(w, "stub.html", struct {
			Title   string
			JoinURL string
		}{Title: title, JoinURL: joinURL}, http.StatusOK)
	}
}

func requireSetup(db *store.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hasHash, err := db.HasAdminHash(r.Context())
		if err != nil {
			http.Error(w, "Could not read setup state.", http.StatusInternalServerError)
			return
		}
		if !hasHash {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
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
	if requestScheme(r) == "https" {
		return true
	}
	return isLoopbackHostname(requestHostname(r))
}

func joinURLForRequest(r *http.Request, fallback string) string {
	if isLoopbackHostname(requestHostname(r)) {
		return fallback
	}
	if r.Host == "" {
		return fallback
	}
	return requestScheme(r) + "://" + r.Host + "/"
}

func requestHostname(r *http.Request) string {
	host := r.Host
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return strings.Trim(host, "[]")
}

func isLoopbackHostname(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	proto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	if strings.EqualFold(proto, "https") {
		return "https"
	}
	return "http"
}
