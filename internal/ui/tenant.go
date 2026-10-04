package ui

import (
	"bytes"
	"html/template"
	"io"
	"net/http"
)

// Shell surfaces. /board is the TV shell and / is the phone shell.
const (
	SurfaceBoard = "board"
	SurfacePhone = "phone"
)

// TenantTrigger is the HX-Trigger event name a response sends when the
// request changed which tenant, or which version of it, the shell should show.
// The shell refetches its tenant when it hears it.
const TenantTrigger = "grabbag:tenant"

// Tenant is a screen the persistent /board and / shells swap in and out: the
// Lobby, every game, and the host's locked board. Board and Phone write HTML
// fragments, never full documents. The shell owns the document, the SSE
// connection, theme, notices, the connection overlay, and the heartbeat.
type Tenant interface {
	// Board writes the TV fragment.
	Board(w http.ResponseWriter, r *http.Request)
	// Phone writes the phone fragment.
	Phone(w http.ResponseWriter, r *http.Request)
	// Assets lists the tenant's stylesheets and scripts. They cover both
	// surfaces. The shell loads each once and removes the CSS on unmount.
	Assets() Assets
	// Scenarios lists preview states. Host wraps each in the static shell.
	Scenarios() []Scenario
}

// Assets is a tenant's browser files. JS files call grabbagShell.register.
// External is third-party stylesheets such as web fonts. Those stay loaded
// once the shell has them.
type Assets struct {
	CSS      []string `json:"css"`
	JS       []string `json:"js"`
	External []string `json:"external"`
}

// Shell is one persistent shell document with its first tenant already
// rendered inside it. Static renders a frozen preview: no htmx, no SSE, no
// heartbeat, no transitions, and no tenant scripts, so nothing mounts. The
// shell still sets the viewport height and size class.
type Shell struct {
	Chrome
	Surface    string
	Tenant     string
	Generation uint64
	Boot       string
	// Stream is the SSE URL the shell connects to.
	Stream string
	// Player is true when a phone shell rendered for a signed-in player.
	Player bool
	Assets Assets
	Body   template.HTML
}

var shellTemplates = template.Must(template.New("shell").Funcs(Funcs()).ParseFS(chromeTemplates, "templates/chrome.html", "templates/shell.html"))

// RenderShell writes the shell document s.
func RenderShell(w io.Writer, s Shell) error {
	var buf bytes.Buffer
	if err := shellTemplates.ExecuteTemplate(&buf, "ui-shell", s); err != nil {
		return err
	}
	_, err := buf.WriteTo(w)
	return err
}

// StaticShell is the frozen preview shell around one scenario fragment. It
// links the tenant's stylesheets and no scripts, so nothing mounts.
func StaticShell(surface, tenant string, p Preview, assets Assets, body template.HTML) Shell {
	return Shell{
		Chrome:  Chrome{Title: "GrabBag.gg preview", Theme: NormalizeTheme(p.Theme), Static: true, OpenIDs: p.Open},
		Surface: surface,
		Tenant:  tenant,
		Assets:  Assets{CSS: assets.CSS, External: assets.External},
		Body:    body,
	}
}

// ShellSurface is the shell a scenario's Shell value renders inside, or ""
// when the scenario is not a shell tenant fragment.
func ShellSurface(shell string) string {
	switch shell {
	case ShellBoard:
		return SurfaceBoard
	case ShellPhone, ShellPlayPhone:
		return SurfacePhone
	}
	return ""
}
