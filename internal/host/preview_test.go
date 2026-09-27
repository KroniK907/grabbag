package host

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/games"
)

func previewGet(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, string(body)
}

func TestPreviewIndexListsLobbyAndGames(t *testing.T) {
	t.Parallel()
	h := newPreviewHandler(games.Catalog())
	code, body := previewGet(t, h, "/dev/ui/index.json")
	if code != http.StatusOK {
		t.Fatalf("index = %d", code)
	}
	var doc PreviewIndex
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatal(err)
	}
	pkgs := map[string]int{}
	samples := 0
	for _, s := range doc.Scenarios {
		pkgs[s.Package]++
		if s.Sample {
			samples++
		}
	}
	for _, id := range []string{"lobby", "apples", "quips", "testing"} {
		if pkgs[id] == 0 {
			t.Errorf("index has no %s scenarios", id)
		}
	}
	if samples == 0 || len(doc.Devices) == 0 || len(doc.TextScales) == 0 {
		t.Fatalf("samples=%d devices=%d scales=%d", samples, len(doc.Devices), len(doc.TextScales))
	}
	code, body = previewGet(t, h, "/dev/ui/")
	if code != http.StatusOK || !strings.Contains(body, `id="game"`) || !strings.Contains(body, "All games") {
		t.Fatalf("gallery game filter missing, status %d", code)
	}
}

func TestPreviewScenarioWrapsGamePhoneAndAppliesPatch(t *testing.T) {
	t.Parallel()
	h := newPreviewHandler(games.Catalog())
	q := url.Values{"players": {"64"}, "theme": {"neon-dark"}, "patch": {`{"Error":"patched error text"}`}}
	code, body := previewGet(t, h, "/dev/ui/s/apples/phone/submit-seated?"+q.Encode())
	if code != http.StatusOK {
		t.Fatalf("scenario = %d %s", code, body)
	}
	for _, want := range []string{"patched error text", `data-theme="neon-dark"`, "ui-phone", "/dev/ui/assets/apples/game.css", "window.grabbagStatic"} {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
		}
	}
	if strings.Contains(body, "sse-connect") {
		t.Error("preview connects to SSE")
	}
}

func TestPreviewSettingsShellAndAssets(t *testing.T) {
	t.Parallel()
	h := newPreviewHandler(games.Catalog())
	code, body := previewGet(t, h, "/dev/ui/s/quips/settings/match")
	if code != http.StatusOK || !strings.Contains(body, `id="game-settings"`) || !strings.Contains(body, "settings-knobs") {
		t.Fatalf("settings shell = %d", code)
	}
	if code, _ := previewGet(t, h, "/dev/ui/assets/quips/game.css"); code != http.StatusOK {
		t.Fatalf("asset = %d", code)
	}
	if code, _ := previewGet(t, h, "/dev/ui/s/quips/settings/nope"); code != http.StatusNotFound {
		t.Fatalf("unknown scenario = %d", code)
	}
}

func TestPreviewTableShowsEveryViewer(t *testing.T) {
	t.Parallel()
	h := newPreviewHandler(games.Catalog())
	code, body := previewGet(t, h, "/dev/ui/table/apples/reveal?players=12")
	if code != http.StatusOK {
		t.Fatalf("table = %d", code)
	}
	for _, viewer := range []string{"tv", "judge", "seated", "host", "audience"} {
		if !strings.Contains(body, "· "+viewer) {
			t.Errorf("table missing %s", viewer)
		}
	}
	if code, body := previewGet(t, h, "/dev/ui/"); code != http.StatusOK || !strings.Contains(body, "index.json") {
		t.Fatalf("gallery = %d", code)
	}
}

func TestWithPreviewLeavesOtherRoutes(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "host") })
	h := withPreview(next)
	if _, body := previewGet(t, h, "/board"); body != "host" {
		t.Fatalf("/board went to %q", body)
	}
	if code, _ := previewGet(t, h, "/dev/ui/index.json"); code != http.StatusOK {
		t.Fatalf("preview index = %d", code)
	}
}
