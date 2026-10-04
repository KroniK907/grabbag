package host

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/lobby"
	"github.com/KroniK907/grabbag/internal/ui"
)

// PreviewPath is where the -dev-preview UI gallery mounts.
const PreviewPath = "/dev/ui/"

// PreviewDevice is one emulated screen for the gallery and grabbag-uishots.
type PreviewDevice struct {
	ID     string  `json:"id"`
	Label  string  `json:"label"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Scale  float64 `json:"scale"`
	Mobile bool    `json:"mobile"`
	Tablet bool    `json:"tablet"`
	// UA is "ios" or "android". Empty is desktop.
	UA string `json:"ua"`
}

// PreviewDevices are portrait sizes in CSS pixels. Tablets also run landscape.
var PreviewDevices = []PreviewDevice{
	{ID: "iphone-se", Label: "iPhone SE", Width: 375, Height: 667, Scale: 2, Mobile: true, UA: "ios"},
	{ID: "iphone-13-mini", Label: "iPhone 13 mini", Width: 375, Height: 812, Scale: 3, Mobile: true, UA: "ios"},
	{ID: "iphone-16", Label: "iPhone 15/16", Width: 393, Height: 852, Scale: 3, Mobile: true, UA: "ios"},
	{ID: "iphone-16-pro-max", Label: "iPhone 15/16 Pro Max", Width: 430, Height: 932, Scale: 3, Mobile: true, UA: "ios"},
	{ID: "galaxy-a", Label: "Galaxy A series", Width: 360, Height: 800, Scale: 3, Mobile: true, UA: "android"},
	{ID: "pixel-8", Label: "Pixel 8", Width: 412, Height: 915, Scale: 2.625, Mobile: true, UA: "android"},
	{ID: "galaxy-s24-ultra", Label: "Galaxy S24 Ultra", Width: 384, Height: 824, Scale: 3.75, Mobile: true, UA: "android"},
	{ID: "android-small", Label: "Small Android", Width: 320, Height: 640, Scale: 2, Mobile: true, UA: "android"},
	{ID: "ipad-mini", Label: "iPad mini", Width: 744, Height: 1133, Scale: 2, Mobile: true, Tablet: true, UA: "ios"},
	{ID: "ipad-10", Label: "iPad 10th gen", Width: 820, Height: 1180, Scale: 2, Mobile: true, Tablet: true, UA: "ios"},
	{ID: "ipad-pro-13", Label: "iPad Pro 13\"", Width: 1032, Height: 1376, Scale: 2, Mobile: true, Tablet: true, UA: "ios"},
	{ID: "galaxy-tab-s9", Label: "Galaxy Tab S9", Width: 800, Height: 1280, Scale: 2, Mobile: true, Tablet: true, UA: "android"},
}

// PreviewScreens are the big-screen viewports for phone scenarios, next to
// the phone reference frame: iPad portrait and landscape, and a desktop
// browser. The phone frame goes edge to edge on all of them.
var PreviewScreens = []PreviewDevice{
	{ID: "ipad-portrait", Label: "iPad portrait", Width: 820, Height: 1180, Scale: 2, Mobile: true, Tablet: true, UA: "ios"},
	{ID: "ipad-landscape", Label: "iPad landscape", Width: 1180, Height: 820, Scale: 2, Mobile: true, Tablet: true, UA: "ios"},
	{ID: "desktop", Label: "Desktop", Width: 1440, Height: 900, Scale: 1},
}

// PreviewTextScales are the browser text sizes the device matrix runs.
var PreviewTextScales = []float64{1, 1.15, 1.3, 1.5, 2}

// PreviewFrames are the reference viewports per ui.Frame*.
// tv-720 and tv-480 are extra gallery sizes. Scenarios still use FrameTV (1080p).
var PreviewFrames = map[string]PreviewDevice{
	ui.FramePhone: {ID: "phone", Label: "Phone", Width: 393, Height: 852, Scale: 3, Mobile: true, UA: "ios"},
	ui.FrameTV:    {ID: "tv", Label: "TV 1080p", Width: 1920, Height: 1080, Scale: 1},
	"tv-720":      {ID: "tv-720", Label: "TV 720p", Width: 1280, Height: 720, Scale: 1},
	"tv-480":      {ID: "tv-480", Label: "TV 480p", Width: 854, Height: 480, Scale: 1},
	ui.FramePage:  {ID: "page", Label: "Laptop", Width: 1280, Height: 800, Scale: 1},
}

// PreviewScenario is one index.json row.
type PreviewScenario struct {
	Package          string `json:"package"`
	Surface          string `json:"surface"`
	Group            string `json:"group,omitempty"`
	Name             string `json:"name"`
	Viewer           string `json:"viewer"`
	Frame            string `json:"frame"`
	Sample           bool   `json:"sample,omitempty"`
	Players          []int  `json:"players,omitempty"`
	DefaultPlayers   int    `json:"defaultPlayers,omitempty"`
	RealisticPlayers int    `json:"realisticPlayers,omitempty"`
	Path             string `json:"path"`
}

// PreviewIndex is the GET /dev/ui/index.json document.
type PreviewIndex struct {
	Scenarios  []PreviewScenario        `json:"scenarios"`
	Devices    []PreviewDevice          `json:"devices"`
	Screens    []PreviewDevice          `json:"screens"`
	TextScales []float64                `json:"textScales"`
	Frames     map[string]PreviewDevice `json:"frames"`
	Themes     []string                 `json:"themes"`
}

type previewPackage struct {
	id        string
	name      string
	scenarios []ui.Scenario
	// static serves the package's static/ files at /dev/ui/assets/<id>/.
	static http.Handler
	// tenant is the package's shell asset list, linked in the static shell.
	tenant ui.Assets
}

type previewServer struct {
	packages []previewPackage
}

// PreviewHandler serves the UI gallery at PreviewPath and host /static/
// assets. It needs no store: every page renders from package scenarios.
// Mount it only for -dev-preview or tools such as grabbag-uishots.
func PreviewHandler() http.Handler {
	return newPreviewHandler(games.Catalog())
}

// withPreview routes PreviewPath to PreviewHandler and everything else to next.
func withPreview(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(PreviewPath, PreviewHandler())
	mux.Handle("/", next)
	return mux
}

func newPreviewHandler(catalog []games.Factory) http.Handler {
	s := newPreviewServer(catalog)
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", ui.StaticHandler()))
	mux.Handle("GET /lobby/static/", lobby.StaticHandler())
	mux.HandleFunc("GET /dev/ui/{$}", s.gallery)
	mux.HandleFunc("GET /dev/ui/index.json", s.index)
	mux.HandleFunc("GET /dev/ui/s/{pkg}/{surface}/{name}", s.scenario)
	mux.HandleFunc("POST /dev/ui/s/{pkg}/{surface}/{name}", s.scenario)
	mux.HandleFunc("GET /dev/ui/table/{pkg}/{group}", s.table)
	mux.HandleFunc("GET /dev/ui/assets/{pkg}/{file...}", s.asset)
	return mux
}

func (s *previewServer) find(pkgID, surface, name string) (previewPackage, ui.Scenario, bool) {
	for _, pkg := range s.packages {
		if pkg.id != pkgID {
			continue
		}
		for _, sc := range pkg.scenarios {
			if sc.Surface == surface && sc.Name == name {
				return pkg, sc, true
			}
		}
	}
	return previewPackage{}, ui.Scenario{}, false
}

func previewScenarioPath(pkg string, sc ui.Scenario) string {
	return PreviewPath + "s/" + pkg + "/" + sc.Surface + "/" + url.PathEscape(sc.Name)
}

func (s *previewServer) index(w http.ResponseWriter, r *http.Request) {
	doc := PreviewIndex{
		Devices: PreviewDevices, Screens: PreviewScreens, TextScales: PreviewTextScales, Frames: PreviewFrames,
		Themes: []string{ui.ThemeNeonLight, ui.ThemeNeonDark},
	}
	for _, pkg := range s.packages {
		for _, sc := range pkg.scenarios {
			doc.Scenarios = append(doc.Scenarios, PreviewScenario{
				Package: pkg.id, Surface: sc.Surface, Group: sc.Group, Name: sc.Name, Viewer: sc.Viewer,
				Frame: sc.Frame, Sample: sc.Sample, Players: sc.Sweep(),
				DefaultPlayers: sc.DefaultPlayers(), RealisticPlayers: sc.RealisticPlayers(),
				Path: previewScenarioPath(pkg.id, sc),
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doc)
}

// scenario renders one page. Query: players, theme, open, patch (JSON).
// A POST body is one more JSON merge patch, applied last.
func (s *previewServer) scenario(w http.ResponseWriter, r *http.Request) {
	pkg, sc, ok := s.find(r.PathValue("pkg"), r.PathValue("surface"), r.PathValue("name"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	patches := append([][]byte(nil), sc.Patches...)
	if q := r.URL.Query().Get("patch"); q != "" {
		patches = append(patches, []byte(q))
	}
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "Could not read the patch.", http.StatusBadRequest)
			return
		}
		patches = append(patches, body)
	}
	players, _ := strconv.Atoi(r.URL.Query().Get("players"))
	p := ui.Preview{
		Theme:   r.URL.Query().Get("theme"),
		Players: sc.ClampPlayers(players),
		Patches: patches,
		Assets:  PreviewPath + "assets/" + pkg.id + "/",
		Open:    r.URL.Query().Get("open"),
	}
	var page bytes.Buffer
	if err := renderPreview(&page, sc, p, pkg); err != nil {
		http.Error(w, "Preview failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = page.WriteTo(w)
}

// renderPreview writes one scenario page. Board and phone fragments go in
// the static shell with the package's CSS, the way the live shells show them
// but with no scripts and no mount. A phone gets the frame a live phone would:
// Leave, Help for a game, and the host panel for a host viewer.
func renderPreview(w io.Writer, sc ui.Scenario, p ui.Preview, pkg previewPackage) error {
	if sc.Shell == "" {
		return sc.Render(w, p)
	}
	inner := p
	inner.Open = ""
	var body bytes.Buffer
	if err := sc.Render(&body, inner); err != nil {
		return err
	}
	switch sc.Shell {
	case ui.ShellSettings:
		return lobby.RenderSettings(w, p, pkg.id, template.HTML(body.String()))
	case ui.ShellPlayPhone:
		var wrapped bytes.Buffer
		if err := lobby.RenderPlayPhone(&wrapped, p, template.HTML(body.String())); err != nil {
			return err
		}
		body = wrapped
	}
	if ui.ShellSurface(sc.Shell) == "" {
		_, err := body.WriteTo(w)
		return err
	}
	var host bytes.Buffer
	switch {
	case sc.HostPanel != nil:
		if err := sc.HostPanel(&host, inner); err != nil {
			return err
		}
	case sc.Shell == ui.ShellPlayPhone && sc.Viewer == "host":
		if err := lobby.RenderHostPanel(&host, inner); err != nil {
			return err
		}
	}
	shell := p
	if shell.Open == "" {
		shell.Open = sc.Open
	}
	return ui.RenderShell(w, sc.ScenarioShell(pkg.id, shell, pkg.tenant, template.HTML(body.String()), template.HTML(host.String())))
}

// previewAssets points a game's /games/<id>/static/ links at the gallery's copy.
func previewAssets(id string, a ui.Assets) ui.Assets {
	rewrite := func(list []string) []string {
		out := make([]string, 0, len(list))
		for _, u := range list {
			if rest, ok := strings.CutPrefix(u, games.StaticPath(id)); ok {
				u = PreviewPath + "assets/" + id + "/" + rest
			}
			out = append(out, u)
		}
		return out
	}
	return ui.Assets{CSS: rewrite(a.CSS), JS: rewrite(a.JS), Layout: rewrite(a.Layout), External: a.External}
}

func (s *previewServer) asset(w http.ResponseWriter, r *http.Request) {
	for _, pkg := range s.packages {
		if pkg.id == r.PathValue("pkg") && pkg.static != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/static/" + r.PathValue("file")
			r2.URL.RawPath = ""
			pkg.static.ServeHTTP(w, r2)
			return
		}
	}
	http.NotFound(w, r)
}

type previewFrameView struct {
	Label      string
	Src        string
	Width      int
	Height     int
	Zoom       float64
	ClipWidth  int
	ClipHeight int
}

type previewTableView struct {
	Title   string
	Players int
	Theme   string
	Frames  []previewFrameView
}

// table shows every scenario in one group side by side: the TV and each
// viewer's phone at the same moment and table size.
func (s *previewServer) table(w http.ResponseWriter, r *http.Request) {
	pkgID, group := r.PathValue("pkg"), r.PathValue("group")
	players, _ := strconv.Atoi(r.URL.Query().Get("players"))
	theme := ui.NormalizeTheme(r.URL.Query().Get("theme"))
	view := previewTableView{Title: pkgID + " / " + group, Theme: theme}
	var list []ui.Scenario
	for _, pkg := range s.packages {
		if pkg.id != pkgID {
			continue
		}
		for _, sc := range pkg.scenarios {
			if sc.Group == group && !strings.Contains(sc.Name, "~") {
				list = append(list, sc)
			}
		}
	}
	if len(list) == 0 {
		http.NotFound(w, r)
		return
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].Frame == ui.FrameTV && list[j].Frame != ui.FrameTV })
	for _, sc := range list {
		n := sc.ClampPlayers(players)
		if view.Players == 0 {
			view.Players = n
		}
		frame := PreviewFrames[sc.Frame]
		zoom := 1.0
		if frame.Width > 1000 {
			zoom = 0.5
		}
		q := url.Values{"theme": {theme}}
		if n > 0 {
			q.Set("players", strconv.Itoa(n))
		}
		view.Frames = append(view.Frames, previewFrameView{
			Label: sc.Surface + " · " + sc.Viewer, Src: previewScenarioPath(pkgID, sc) + "?" + q.Encode(),
			Width: frame.Width, Height: frame.Height, Zoom: zoom,
			ClipWidth: int(float64(frame.Width) * zoom), ClipHeight: int(float64(frame.Height) * zoom),
		})
	}
	s.render(w, "preview-table.html", view)
}

func (s *previewServer) gallery(w http.ResponseWriter, r *http.Request) {
	s.render(w, "preview-gallery.html", nil)
}

func (s *previewServer) render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "Could not render the preview page.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// newPreviewServer lists the Lobby, the locked board, and every catalog
// game with their scenarios and assets.
func newPreviewServer(catalog []games.Factory) *previewServer {
	s := &previewServer{}
	s.packages = append(s.packages,
		previewPackage{id: lobbyTenantID, name: "Lobby", scenarios: lobby.Scenarios(), tenant: (&lobby.Lobby{}).Assets()},
		previewPackage{id: lockedTenantID, name: "Locked board", scenarios: lockedTenant{}.Scenarios()},
	)
	for _, f := range catalog {
		g := f.New()
		pkg := previewPackage{id: f.ID, name: g.Name(), scenarios: g.Scenarios(), static: g.Play()}
		pkg.tenant = previewAssets(f.ID, g.Assets())
		s.packages = append(s.packages, pkg)
	}
	return s
}
