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

// PreviewTextScales are the browser text sizes the device matrix runs.
var PreviewTextScales = []float64{1, 1.15, 1.3, 1.5, 2}

// PreviewFrames are the reference viewports per ui.Frame*.
var PreviewFrames = map[string]PreviewDevice{
	ui.FramePhone: {ID: "phone", Label: "Phone", Width: 393, Height: 852, Scale: 3, Mobile: true, UA: "ios"},
	ui.FrameTV:    {ID: "tv", Label: "TV 1080p", Width: 1920, Height: 1080, Scale: 1},
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
	TextScales []float64                `json:"textScales"`
	Frames     map[string]PreviewDevice `json:"frames"`
	Themes     []string                 `json:"themes"`
}

type previewPackage struct {
	id        string
	name      string
	scenarios []ui.Scenario
	assets    http.Handler
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
	s := &previewServer{}
	s.packages = append(s.packages, previewPackage{id: "lobby", name: "Lobby", scenarios: lobby.Scenarios()})
	for _, f := range catalog {
		g := f.New()
		pv, ok := g.(games.Previewer)
		if !ok {
			continue
		}
		s.packages = append(s.packages, previewPackage{id: f.ID, name: g.Name(), scenarios: pv.Scenarios(), assets: g.Play()})
	}
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", ui.StaticHandler()))
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
		Devices: PreviewDevices, TextScales: PreviewTextScales, Frames: PreviewFrames,
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
	if err := renderPreview(&page, sc, p, pkg.id); err != nil {
		http.Error(w, "Preview failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = page.WriteTo(w)
}

func renderPreview(w io.Writer, sc ui.Scenario, p ui.Preview, pkgID string) error {
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
	case ui.ShellPlayPhone:
		return lobby.RenderPlayPhone(w, p, sc.Viewer, template.HTML(body.String()))
	case ui.ShellSettings:
		return lobby.RenderSettings(w, p, pkgID, template.HTML(body.String()))
	default:
		_, err := body.WriteTo(w)
		return err
	}
}

func (s *previewServer) asset(w http.ResponseWriter, r *http.Request) {
	for _, pkg := range s.packages {
		if pkg.id == r.PathValue("pkg") && pkg.assets != nil {
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/static/" + r.PathValue("file")
			r2.URL.RawPath = ""
			pkg.assets.ServeHTTP(w, r2)
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
