// Package uitest checks that preview scenarios render. Test code only.
package uitest

import (
	"bytes"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/ui"
)

// RenderAll renders every scenario at each Sweep count (or once without a
// player dimension) in both themes and fails on any error or empty page.
func RenderAll(t *testing.T, list []ui.Scenario) {
	t.Helper()
	if len(list) == 0 {
		t.Fatal("no scenarios")
	}
	seen := map[string]bool{}
	for _, s := range list {
		key := s.Surface + "/" + s.Name
		if seen[key] {
			t.Errorf("duplicate scenario %s", key)
		}
		seen[key] = true
		if s.Render == nil || s.Surface == "" || s.Name == "" || s.Frame == "" {
			t.Errorf("scenario %s is missing Render, Surface, Name, or Frame", key)
			continue
		}
		counts := s.Sweep()
		if counts == nil {
			counts = []int{0}
		}
		for _, n := range counts {
			for _, theme := range []string{ui.ThemeNeonLight, ui.ThemeNeonDark} {
				var buf bytes.Buffer
				p := ui.Preview{Theme: theme, Players: n, Patches: s.Patches, Assets: "/dev/ui/assets/x/"}
				if err := s.Render(&buf, p); err != nil {
					t.Errorf("%s players=%d theme=%s: %v", key, n, theme, err)
					continue
				}
				body := buf.String()
				if strings.TrimSpace(body) == "" {
					t.Errorf("%s players=%d: empty render", key, n)
				}
				if s.Shell == "" && !strings.Contains(body, "window.grabbagStatic") {
					t.Errorf("%s players=%d: full document is not static", key, n)
				}
				if strings.Contains(body, `sse-connect=`) {
					t.Errorf("%s players=%d: preview connects to SSE", key, n)
				}
			}
		}
	}
}
