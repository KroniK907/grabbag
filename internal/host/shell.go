package host

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

// Shell tenant ids the host owns. Games use their catalog id.
const (
	lobbyTenantID  = "lobby"
	lockedTenantID = "locked"
)

// Shell SSE streams. A locked board only hears tenant changes and theme
// flips, so it never sees the roster, the log, or a notice.
const (
	streamRoom   = "/lobby/events"
	streamLocked = "/shell/events/locked"
)

// tenantFrame is the GET /tenant and GET /board/tenant body. The shell swaps
// html in when tenant or generation moved on, and reloads when boot changed.
type tenantFrame struct {
	Tenant     string    `json:"tenant"`
	Generation uint64    `json:"generation"`
	Boot       string    `json:"boot"`
	Theme      string    `json:"theme"`
	Notices    string    `json:"notices"`
	Stream     string    `json:"stream"`
	Player     bool      `json:"player"`
	Kicked     bool      `json:"kicked"`
	Assets     ui.Assets `json:"assets"`
	HTML       string    `json:"html"`
}

// shellTenant is the tenant one request sees on one surface.
type shellTenant struct {
	id     string
	tenant ui.Tenant
	game   games.Game
	stream string
	player bool
	kicked bool
}

func newBootID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("host: boot id: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// bump moves the tenant generation on. Call it after any change to what a
// shell shows, before telling shells to refetch.
func (rt *runtime) bump() { rt.generation.Add(1) }

func (rt *runtime) registerShellRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", rt.shellDocument(ui.SurfacePhone))
	mux.HandleFunc("GET /board", rt.shellDocument(ui.SurfaceBoard))
	mux.HandleFunc("GET /tenant", rt.shellTenantFrame(ui.SurfacePhone))
	mux.HandleFunc("GET /board/tenant", rt.shellTenantFrame(ui.SurfaceBoard))
	mux.Handle("GET "+streamLocked, rt.events.Only("tenant", "theme"))
	mux.HandleFunc("POST /shell/error", rt.postShellError)
}

// locked reports whether r gets the locked board: admin-only board is on and
// r has no admin session. A failed read locks, so the board never leaks.
func (rt *runtime) locked(r *http.Request) bool {
	on, err := rt.room.AdminOnlyBoard(r.Context())
	if err != nil {
		return true
	}
	return on && !rt.hasAdmin(r)
}

func (rt *runtime) boardTenant(r *http.Request) shellTenant {
	if rt.locked(r) {
		return shellTenant{id: lockedTenantID, tenant: lockedTenant{}, stream: streamLocked}
	}
	if !rt.room.RestorePending() {
		rt.mu.Lock()
		started, game := rt.started, rt.game
		rt.mu.Unlock()
		if started && game != nil {
			return shellTenant{id: game.ID(), tenant: game, game: game, stream: streamRoom}
		}
	}
	return shellTenant{id: lobbyTenantID, tenant: rt.room, stream: streamRoom}
}

func (rt *runtime) phoneTenant(r *http.Request) shellTenant {
	_, ok, err := rt.room.PlayerFromRequest(r)
	signedIn := err == nil && ok
	rt.mu.Lock()
	started, game := rt.started, rt.game
	rt.mu.Unlock()
	if signedIn && started && game != nil {
		return shellTenant{id: game.ID(), tenant: game, game: game, stream: streamRoom, player: true}
	}
	return shellTenant{
		id: lobbyTenantID, tenant: rt.room, stream: streamRoom,
		player: signedIn, kicked: !signedIn && rt.room.Kicked(r),
	}
}

func (rt *runtime) pickTenant(surface string, r *http.Request) shellTenant {
	if surface == ui.SurfaceBoard {
		return rt.boardTenant(r)
	}
	return rt.phoneTenant(r)
}

// renderTenant captures the tenant's fragment for surface. A game phone is
// wrapped in the Lobby play-phone fragment (Leave and the host drawer).
func (rt *runtime) renderTenant(surface string, st shellTenant, r *http.Request) (template.HTML, error) {
	var buf bytes.Buffer
	rec := &capture{buf: &buf, header: make(http.Header)}
	switch {
	case surface == ui.SurfaceBoard:
		st.tenant.Board(rec, r)
	case st.game != nil:
		var inner bytes.Buffer
		body := &capture{buf: &inner, header: make(http.Header)}
		st.game.Phone(body, r)
		if body.status != 0 && body.status != http.StatusOK {
			return "", fmt.Errorf("%s phone returned %d", st.id, body.status)
		}
		rt.room.WritePlayPhone(rec, r, template.HTML(inner.String()))
	default:
		st.tenant.Phone(rec, r)
	}
	if rec.status != 0 && rec.status != http.StatusOK {
		return "", fmt.Errorf("%s %s returned %d", st.id, surface, rec.status)
	}
	return template.HTML(buf.String()), nil
}

func shellTitle(surface string) string {
	if surface == ui.SurfaceBoard {
		return "GrabBag.gg board"
	}
	return "GrabBag.gg"
}

// shellDocument serves GET / and GET /board: the shell with the current
// tenant already rendered inside it.
func (rt *runtime) shellDocument(surface string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read the generation first, so the tenant is at least this new.
		gen := rt.generation.Load()
		st := rt.pickTenant(surface, r)
		body, err := rt.renderTenant(surface, st, r)
		if err != nil {
			rt.log.Write("shell: " + err.Error())
			http.Error(w, "Could not render the page.", http.StatusInternalServerError)
			return
		}
		var page bytes.Buffer
		err = ui.RenderShell(&page, ui.Shell{
			Chrome: ui.Chrome{
				Title:         shellTitle(surface),
				Theme:         rt.room.Theme(r.Context()),
				NoticeTargets: rt.room.NoticeTargets(r),
			},
			Surface:    surface,
			Tenant:     st.id,
			Generation: gen,
			Boot:       rt.boot,
			Stream:     st.stream,
			Player:     st.player,
			Assets:     st.tenant.Assets(),
			Body:       body,
		})
		if err != nil {
			http.Error(w, "Could not render the page.", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = page.WriteTo(w)
	}
}

// shellTenantFrame serves GET /tenant and GET /board/tenant for transitions.
// It uses the same cookie and admin checks as the shell document.
func (rt *runtime) shellTenantFrame(surface string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gen := rt.generation.Load()
		st := rt.pickTenant(surface, r)
		body, err := rt.renderTenant(surface, st, r)
		if err != nil {
			rt.log.Write("shell: " + err.Error())
			http.Error(w, "Could not render the tenant.", http.StatusInternalServerError)
			return
		}
		frame := tenantFrame{
			Tenant:     st.id,
			Generation: gen,
			Boot:       rt.boot,
			Theme:      rt.room.Theme(r.Context()),
			Notices:    rt.room.NoticeTargets(r),
			Stream:     st.stream,
			Player:     st.player,
			Kicked:     st.kicked,
			Assets:     st.tenant.Assets(),
			HTML:       string(body),
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(frame)
	}
}

// postShellError records a shell or tenant script error in the host log so
// it shows on /settings/log. It stores nothing else.
func (rt *runtime) postShellError(w http.ResponseWriter, r *http.Request) {
	var report struct {
		Surface string `json:"surface"`
		Tenant  string `json:"tenant"`
		Phase   string `json:"phase"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&report); err != nil {
		http.Error(w, "Could not read the report.", http.StatusBadRequest)
		return
	}
	clean := func(s string, n int) string {
		s = strings.Join(strings.Fields(s), " ")
		if len(s) > n {
			s = s[:n] + "…"
		}
		return s
	}
	rt.log.Write(fmt.Sprintf("shell error: %s %s %s: %s",
		clean(report.Surface, 16), clean(report.Tenant, 40), clean(report.Phase, 24), clean(report.Message, 400)))
	w.WriteHeader(http.StatusNoContent)
}

// shellDone ends a host POST that changed the tenant. htmx gets 204 and the
// tenant trigger. A plain form post gets a 303 to target.
func shellDone(w http.ResponseWriter, r *http.Request, target string) {
	w.Header().Set("HX-Trigger", ui.TenantTrigger)
	if isHX(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func isHX(r *http.Request) bool { return r.Header.Get("HX-Request") != "" }

// lockedTenant is the admin-only board: a card that says the screen is for
// the host. No scripts, no room data.
type lockedTenant struct{}

func (lockedTenant) Board(w http.ResponseWriter, r *http.Request) {
	renderPage(w, "locked", nil, http.StatusOK)
}

func (lockedTenant) Phone(w http.ResponseWriter, r *http.Request) {
	renderPage(w, "locked", nil, http.StatusOK)
}

func (lockedTenant) Assets() ui.Assets { return ui.Assets{} }

func (lockedTenant) Scenarios() []ui.Scenario {
	return []ui.Scenario{{
		Surface: "board", Group: "locked", Name: "locked", Viewer: "tv", Frame: ui.FrameTV, Shell: ui.ShellBoard,
		Render: func(w io.Writer, p ui.Preview) error {
			return pageTemplates.ExecuteTemplate(w, "locked", nil)
		},
	}}
}
