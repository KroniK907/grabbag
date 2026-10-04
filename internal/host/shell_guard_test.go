package host

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/lobby"
	"github.com/KroniK907/grabbag/internal/ui"
)

var plainPostForm = regexp.MustCompile(`(?i)<form[^>]*\bmethod\s*=\s*"?post`)

// TestNoPlainFormsOrReloadsInsideAShell renders every tenant scenario in its
// shell, the way previews and live shells show it. None may hold a plain
// form post or a location.reload: in-shell actions go through htmx, and the
// only reloads are the shell's hardReload.
func TestNoPlainFormsOrReloadsInsideAShell(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(PreviewHandler())
	t.Cleanup(server.Close)
	var index PreviewIndex
	getJSON(t, server.URL+PreviewPath+"index.json", &index)
	shells := map[string]bool{}
	for _, pkg := range newPreviewHandlerPackages() {
		for _, sc := range pkg.scenarios {
			if ui.ShellSurface(sc.Shell) != "" {
				shells[pkg.id+"/"+sc.Surface+"/"+sc.Name] = true
			}
		}
	}
	checked := 0
	for _, sc := range index.Scenarios {
		if !shells[sc.Package+"/"+sc.Surface+"/"+sc.Name] {
			continue
		}
		page := getText(t, server.URL+sc.Path)
		if m := plainPostForm.FindString(page); m != "" {
			t.Errorf("%s: plain form post inside the shell: %s", sc.Path, m)
		}
		if strings.Contains(page, "location.reload") {
			t.Errorf("%s: location.reload inside the shell", sc.Path)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no shell scenarios checked")
	}
}

// TestTenantScriptsRegisterBothSurfaces reads every tenant's JS and checks
// it registers its own id with a board and a phone lifecycle, and never
// reloads the page itself.
func TestTenantScriptsRegisterBothSurfaces(t *testing.T) {
	t.Parallel()
	type script struct {
		tenant string
		url    string
		serve  http.Handler
	}
	scripts := []script{}
	for _, u := range (&lobby.Lobby{}).Assets().JS {
		scripts = append(scripts, script{tenant: lobbyTenantID, url: u, serve: lobby.StaticHandler()})
	}
	for _, f := range games.Catalog() {
		g := f.New()
		js := g.Assets().JS
		if len(js) == 0 {
			t.Errorf("%s lists no JS asset; every game registers with the shell", f.ID)
		}
		play := http.StripPrefix(strings.TrimSuffix(games.StaticPath(f.ID), "/static/"), g.Play())
		for _, u := range js {
			scripts = append(scripts, script{tenant: f.ID, url: u, serve: play})
		}
	}
	register := func(id string) *regexp.Regexp {
		return regexp.MustCompile(`grabbagShell\.register\(\s*"` + regexp.QuoteMeta(id) + `"`)
	}
	for _, s := range scripts {
		rec := httptest.NewRecorder()
		s.serve.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, s.url, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s: GET %s = %d", s.tenant, s.url, rec.Code)
			continue
		}
		body := rec.Body.String()
		if !register(s.tenant).MatchString(body) {
			t.Errorf("%s: %s does not call grabbagShell.register(%q, ...)", s.tenant, s.url, s.tenant)
		}
		if !regexp.MustCompile(`\bboard\s*:`).MatchString(body) || !regexp.MustCompile(`\bphone\s*:`).MatchString(body) {
			t.Errorf("%s: %s does not register both board and phone", s.tenant, s.url)
		}
		if strings.Contains(body, "location.reload") {
			t.Errorf("%s: %s reloads the page; only grabbagShell.hardReload may", s.tenant, s.url)
		}
	}
}

// TestShellReloadsOnlyThroughHardReload checks shell.js has exactly one
// location.reload, inside hardReload.
func TestShellReloadsOnlyThroughHardReload(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	http.StripPrefix("/static/", ui.StaticHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/shell.js", nil))
	body := rec.Body.String()
	if n := strings.Count(body, "location.reload"); n != 1 {
		t.Fatalf("shell.js has %d location.reload calls, want 1", n)
	}
	fn := body[strings.Index(body, "function hardReload"):]
	fn = fn[:strings.Index(fn, "\n  }\n")]
	if !strings.Contains(fn, "location.reload") {
		t.Fatal("shell.js reloads outside hardReload")
	}
}

func newPreviewHandlerPackages() []previewPackage {
	return newPreviewServer(games.Catalog()).packages
}

func getText(t *testing.T, u string) string {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d %s", u, resp.StatusCode, body)
	}
	return string(body)
}

func getJSON(t *testing.T, u string, out any) {
	t.Helper()
	if err := json.Unmarshal([]byte(getText(t, u)), out); err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
}
