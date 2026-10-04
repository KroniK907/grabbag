package ui_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/ui"
)

func TestStaticPathBustsCache(t *testing.T) {
	t.Parallel()
	got := ui.StaticPath("live.css")
	if got != "/static/live.css?v="+ui.AssetVersion {
		t.Fatalf("StaticPath = %q", got)
	}
	if !strings.Contains(got, "?v=") {
		t.Fatal("static path missing version query")
	}
}

func TestShellPublishesTheVisualViewport(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	http.StripPrefix("/static/", ui.StaticHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/shell.js", nil))
	for _, want := range []string{"visualViewport", "--shell-visual-height", `"shell-short", height < 760`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("shell.js missing %q", want)
		}
	}
}
