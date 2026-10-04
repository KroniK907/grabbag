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
	if err := lobby.RenderPlayPhone(&buf, ui.Preview{Theme: ui.ThemeNeonDark}, "<p>game body</p>"); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	if !strings.Contains(body, "<p>game body</p>") {
		t.Errorf("play phone missing the game body: %s", body)
	}
	// Leave and the host panel are in the shell frame, not the tenant.
	for _, banned := range []string{`id="host-drawer"`, `hx-post="/lobby/leave"`, "sse-connect", "<html"} {
		if strings.Contains(body, banned) {
			t.Errorf("play phone has %q", banned)
		}
	}
}

func TestRenderHostPanelStatic(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := lobby.RenderHostPanel(&buf, ui.Preview{Theme: ui.ThemeNeonDark}); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	for _, want := range []string{`id="host-drawer"`, "ui-drawer-handle", "ui-drawer-collapse", `hx-post="/settings/stop"`} {
		if !strings.Contains(body, want) {
			t.Errorf("host panel missing %q", want)
		}
	}
}
