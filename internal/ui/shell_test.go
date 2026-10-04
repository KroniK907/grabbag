package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/ui"
)

// A static board preview mounts nothing, so it must not load the audio
// engine or draw the sound card (GM-029).
func TestStaticBoardHasNoAudio(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	shell := ui.StaticShell(ui.SurfaceBoard, "lobby", ui.Preview{}, ui.Assets{}, "<div></div>")
	if err := ui.RenderShell(&buf, shell); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"audio.js", "shell-sound", "data-audio"} {
		if strings.Contains(buf.String(), bad) {
			t.Fatalf("static board has %q", bad)
		}
	}
}
