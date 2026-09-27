package apples_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/games/apples"
	"github.com/KroniK907/grabbag/internal/ui"
	"github.com/KroniK907/grabbag/internal/ui/uitest"
)

func TestSubmitPickTwoJudgeShowsTwoBlanks(t *testing.T) {
	t.Parallel()
	body := renderNamed(t, "phone", "submit-pick2-judge")
	if !strings.Contains(body, "____ &#43; ____") {
		t.Fatalf("judge pick-two prompt missing two blanks: %s", snippet(body, "apples-prompt"))
	}
	seated := renderNamed(t, "phone", "submit-pick2-seated")
	if strings.Count(seated, "apples-hole-number") < 2 {
		t.Fatalf("seated pick-two phone has one hole: %s", snippet(seated, "apples-holes"))
	}
}

func TestScoredPreviewShowsJudgePickAndFavorites(t *testing.T) {
	t.Parallel()
	board := renderNamed(t, "board", "scored")
	for _, want := range []string{"Round winner", "Judge's pick", "Favorite 1st"} {
		if !strings.Contains(board, want) {
			t.Fatalf("board scored missing %q", want)
		}
	}
	phone := renderNamed(t, "phone", "scored-audience")
	for _, want := range []string{"Round scoring", "Judge's pick", "Favorite 1st"} {
		if !strings.Contains(phone, want) {
			t.Fatalf("audience scored missing %q: %s", want, snippet(phone, "apples-step"))
		}
	}
}

func renderNamed(t *testing.T, surface, name string) string {
	t.Helper()
	for _, s := range apples.New().Scenarios() {
		if s.Surface == surface && s.Name == name {
			var buf bytes.Buffer
			if err := s.Render(&buf, ui.Preview{Theme: ui.ThemeNeonLight, Players: 4, Assets: "/dev/ui/assets/x/"}); err != nil {
				t.Fatal(err)
			}
			return buf.String()
		}
	}
	t.Fatalf("missing scenario %s/%s", surface, name)
	return ""
}

func snippet(body, needle string) string {
	i := strings.Index(body, needle)
	if i < 0 {
		return body[:min(400, len(body))]
	}
	end := i + 240
	if end > len(body) {
		end = len(body)
	}
	return body[i:end]
}

func TestHostRolePreviews(t *testing.T) {
	t.Parallel()
	settings := renderNamed(t, "settings", "host-roles")
	if !strings.Contains(settings, "Host always judges") || !strings.Contains(settings, "Host reveals answers") {
		t.Fatal("host role settings missing")
	}
	if strings.Count(settings, "checked") < 2 {
		t.Fatal("host role checkboxes are off")
	}
	host := renderNamed(t, "phone", "host-reveal-host")
	if !strings.Contains(host, "apples-host-reveal") || !strings.Contains(host, ">Reveal next<") {
		t.Fatalf("host reveal phone missing the reveal control: %s", snippet(host, "apples-host-reveal"))
	}
	judge := renderNamed(t, "phone", "host-reveal-judge")
	if strings.Contains(judge, ">Reveal next<") {
		t.Fatal("non-host judge can reveal while host reveals is on")
	}
	pinned := renderNamed(t, "phone", "host-judge-judge")
	if !strings.Contains(pinned, "apples-burn-handle") || !strings.Contains(pinned, ">Reveal next<") {
		t.Fatal("pinned host judge is missing judge and host controls")
	}
}

func TestScenariosRenderAtEverySweepCount(t *testing.T) {
	t.Parallel()
	uitest.RenderAll(t, apples.New().Scenarios())
}
