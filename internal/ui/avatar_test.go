package ui_test

import (
	"strings"
	"testing"

	"github.com/KroniK907/hackbox/internal/ui"
)

func TestAvatarSVGIsDeterministicAndSeeded(t *testing.T) {
	t.Parallel()
	a := string(ui.AvatarSVG("seed-one"))
	b := string(ui.AvatarSVG("seed-one"))
	c := string(ui.AvatarSVG("seed-two"))
	if a != b {
		t.Fatal("same seed produced different SVG")
	}
	if a == c {
		t.Fatal("different seeds produced the same SVG")
	}
	if !strings.Contains(a, "<svg") || strings.Contains(a, "<img") {
		t.Fatalf("avatar is not inline SVG: %s", a)
	}
	if ui.SeedColor("seed-one") == ui.SeedColor("seed-two") {
		t.Fatal("different seeds produced the same token color")
	}
	if ui.FaceColor("seed-one") == ui.TokenColor("seed-one") {
		t.Fatal("face and token used the same fill")
	}
	if strings.Contains(a, "L54 12 Q32 0") {
		t.Fatal("avatar still draws the old forehead plate")
	}
}
