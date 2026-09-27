package lobby_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/lobby"
	"github.com/KroniK907/grabbag/internal/ui"
	"github.com/KroniK907/grabbag/internal/ui/uitest"
)

func TestScenariosRenderAtEverySweepCount(t *testing.T) {
	t.Parallel()
	uitest.RenderAll(t, lobby.Scenarios())
}

func TestRenderPlayPhoneWrapsBodyStatic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := lobby.RenderPlayPhone(&buf, ui.Preview{Theme: ui.ThemeNeonDark}, "host", "<p>game body</p>"); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	for _, want := range []string{"<p>game body</p>", "window.grabbagStatic", `data-theme="neon-dark"`, `id="host-drawer"`} {
		if !strings.Contains(body, want) {
			t.Errorf("play phone missing %q", want)
		}
	}
	if strings.Contains(body, "sse-connect") {
		t.Error("static play phone connects to SSE")
	}
}
