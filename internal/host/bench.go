package host

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/KroniK907/grabbag/internal/store"
)

// BenchPath is where the -dev-bench test bench page mounts.
const BenchPath = "/dev/bench/"

// BenchPassword is the admin password of every -dev-bench room.
const BenchPassword = "devbench"

// benchAutofillPath serves the seat script that fills game text boxes.
const benchAutofillPath = BenchPath + "autofill.js"

//go:embed bench_autofill.js
var benchAutofillJS []byte

// Bench seat limits. Seat 1 is always the host.
const (
	BenchMinSeats     = 3
	BenchMaxSeats     = 6
	benchDefaultSeats = 4
	benchCookiePrefix = "grabbag_"
)

// runBench serves a throwaway room for one-screen testing. The main port
// serves the board, /settings, the bench page, and the PreviewPath gallery. Ports port+1 through
// port+BenchMaxSeats each serve one phone seat. Browsers share cookies
// across ports, so each seat port renames the grabbag_ cookies it sees.
// It binds loopback only so a preview proxy can hold the same ports outside.
func runBench(opts options) error {
	base, err := strconv.Atoi(opts.port)
	if err != nil || base+BenchMaxSeats > 65535 {
		return fmt.Errorf("host: -dev-bench needs ports %s through %d", opts.port, base+BenchMaxSeats)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("host: user cache dir: %w", err)
	}
	dataDir := filepath.Join(cache, "grabbag-bench")
	if err := os.RemoveAll(dataDir); err != nil {
		return fmt.Errorf("host: clear bench data: %w", err)
	}
	db, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err := finishBenchSetup(context.Background(), db); err != nil {
		return err
	}
	handler, err := NewHandler(db, "")
	if err != nil {
		return fmt.Errorf("host: build handler: %w", err)
	}
	if _, err := db.SQL().Exec(`UPDATE room_state SET open = 1 WHERE id = 1`); err != nil {
		return fmt.Errorf("host: open bench room: %w", err)
	}

	listeners := make([]net.Listener, 0, BenchMaxSeats+1)
	defer func() {
		for _, l := range listeners {
			_ = l.Close()
		}
	}()
	for i := 0; i <= BenchMaxSeats; i++ {
		l, err := listenOn("127.0.0.1", strconv.Itoa(base+i))
		if err != nil {
			return err
		}
		listeners = append(listeners, l)
	}
	errs := make(chan error, len(listeners))
	for seat := 1; seat <= BenchMaxSeats; seat++ {
		go func(seat int, l net.Listener) {
			errs <- http.Serve(l, benchSeatHandler(seat, handler))
		}(seat, listeners[seat])
	}
	go func() {
		errs <- http.Serve(listeners[0], withPreview(benchMainHandler(handler)))
	}()
	log.Printf("GrabBag.gg test bench http://127.0.0.1:%d%s (admin password %q)", base, BenchPath, BenchPassword)
	return fmt.Errorf("host: serve: %w", <-errs)
}

func finishBenchSetup(ctx context.Context, db *store.DB) error {
	hash, err := hashPassword(BenchPassword)
	if err != nil {
		return err
	}
	sessionID, err := newSessionID()
	if err != nil {
		return err
	}
	return db.FinishSetup(ctx, hash, sessionID)
}

// benchMainHandler adds the bench page and its operator login to the host routes.
func benchMainHandler(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+BenchPath+"{$}", benchPage)
	mux.HandleFunc("GET "+BenchPath+"settings", func(w http.ResponseWriter, r *http.Request) {
		form := url.Values{"password": {BenchPassword}}
		benchFinish(w, r, benchPost(r, next, "/settings/login", form))
	})
	mux.Handle("/", next)
	return mux
}

// benchSeatHandler serves one phone seat. GET /dev/bench/join joins the room
// as that seat when its cookie holds no live player. Seat 1 joins as host.
func benchSeatHandler(seat int, next http.Handler) http.Handler {
	suffix := "_s" + strconv.Itoa(seat)
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+BenchPath+"join", func(w http.ResponseWriter, r *http.Request) {
		form := url.Values{}
		if seat == 1 {
			form.Set("admin_password", BenchPassword)
		}
		// A second browser, or cleared cookies, finds the seat name taken.
		var rec *httptest.ResponseRecorder
		for try := 1; try <= 9; try++ {
			name := benchSeatName(seat)
			if try > 1 {
				name += " (" + strconv.Itoa(try) + ")"
			}
			form.Set("display_name", name)
			rec = benchPost(r, next, "/lobby/join", form)
			if rec.Code != http.StatusConflict {
				break
			}
		}
		benchFinish(w, r, rec)
	})
	mux.HandleFunc("GET "+benchAutofillPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(benchAutofillJS)
	})
	mux.Handle("/", benchInjectAutofill(next))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.Clone(r.Context())
		scopeRequestCookies(r, suffix)
		sw := &seatCookieWriter{ResponseWriter: w, suffix: suffix}
		mux.ServeHTTP(sw, r)
		sw.rename()
	})
}

// benchInjectAutofill adds the autofill script to full HTML page loads.
// htmx swaps and the SSE stream pass through untouched.
func benchInjectAutofill(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("HX-Request") != "" || !strings.Contains(r.Header.Get("Accept"), "text/html") {
			next.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		body := rec.Body.Bytes()
		if strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			tag := []byte(`<script src="` + benchAutofillPath + `" defer></script></body>`)
			body = bytes.Replace(body, []byte("</body>"), tag, 1)
		}
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.Header().Del("Content-Length")
		w.WriteHeader(rec.Code)
		_, _ = w.Write(body)
	})
}

func benchSeatName(seat int) string {
	if seat == 1 {
		return "Host"
	}
	return "Player " + strconv.Itoa(seat)
}

// benchPost replays r as a form POST to path on next, keeping its cookies.
func benchPost(r *http.Request, next http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(r.Context(), http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = r.Host
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range r.Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, req)
	return rec
}

// benchFinish copies the cookies rec set and redirects where it pointed.
func benchFinish(w http.ResponseWriter, r *http.Request, rec *httptest.ResponseRecorder) {
	for _, c := range rec.Result().Cookies() {
		http.SetCookie(w, c)
	}
	target := rec.Header().Get("Location")
	if target == "" {
		http.Error(w, fmt.Sprintf("Bench step failed with status %d.", rec.Code), http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// scopeRequestCookies keeps only this seat's grabbag_ cookies, under their plain names.
func scopeRequestCookies(r *http.Request, suffix string) {
	cookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, c := range cookies {
		if strings.HasPrefix(c.Name, benchCookiePrefix) {
			name, ok := strings.CutSuffix(c.Name, suffix)
			if !ok {
				continue
			}
			c.Name = name
		}
		r.AddCookie(c)
	}
}

// seatCookieWriter renames grabbag_ cookies the host sets to this seat's names.
type seatCookieWriter struct {
	http.ResponseWriter
	suffix string
	done   bool
}

func (w *seatCookieWriter) rename() {
	if w.done {
		return
	}
	w.done = true
	h := w.Header()
	lines := h.Values("Set-Cookie")
	h.Del("Set-Cookie")
	for _, line := range lines {
		if c, err := http.ParseSetCookie(line); err == nil && strings.HasPrefix(c.Name, benchCookiePrefix) {
			c.Name += w.suffix
			line = c.String()
		}
		h.Add("Set-Cookie", line)
	}
}

func (w *seatCookieWriter) WriteHeader(code int) {
	w.rename()
	w.ResponseWriter.WriteHeader(code)
}

func (w *seatCookieWriter) Write(b []byte) (int, error) {
	w.rename()
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController flush SSE through the wrapper.
func (w *seatCookieWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush keeps the http.Flusher that the SSE hub checks for.
func (w *seatCookieWriter) Flush() {
	w.rename()
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type benchSeatView struct {
	Seat int
	Name string
	Port int
}

type benchView struct {
	Seats    []benchSeatView
	Count    int
	Choices  []int
	Password string
}

func benchPage(w http.ResponseWriter, r *http.Request) {
	count := benchDefaultSeats
	if n, err := strconv.Atoi(r.URL.Query().Get("players")); err == nil {
		count = min(max(n, BenchMinSeats), BenchMaxSeats)
	}
	_, portText, err := net.SplitHostPort(r.Host)
	base, perr := strconv.Atoi(portText)
	if err != nil || perr != nil {
		base, _ = strconv.Atoi(listenPort)
	}
	view := benchView{Count: count, Password: BenchPassword}
	for n := BenchMinSeats; n <= BenchMaxSeats; n++ {
		view.Choices = append(view.Choices, n)
	}
	for seat := 1; seat <= count; seat++ {
		view.Seats = append(view.Seats, benchSeatView{Seat: seat, Name: benchSeatName(seat), Port: base + seat})
	}
	renderPage(w, "bench.html", view, http.StatusOK)
}
