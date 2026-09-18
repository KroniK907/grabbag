package quips

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanSkipsStateSubdirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "state"), 0o700); err != nil {
		t.Fatal(err)
	}
	hidden := `{"formatVersion":1,"id":"hidden","name":"Hidden","packs":[{"id":"hidden","name":"Hidden","prompts":[{"id":"p1","text":"Hi"}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "state", "hidden.json"), []byte(hidden), 0o600); err != nil {
		t.Fatal(err)
	}
	cat := scanDataDir(dir)
	if len(cat.Libraries) != 0 || len(cat.Failed) != 0 {
		t.Fatalf("state/ was scanned: libs=%d failed=%d", len(cat.Libraries), len(cat.Failed))
	}
}

func TestReconcileNewPackDefaultsOff(t *testing.T) {
	t.Parallel()
	existing := factorySettings()
	existing.setPack("quips", "comedy", true)
	cat := catalog{Libraries: []libraryFile{{
		ID: "quips", Name: "Quick Quips",
		Packs: []packFile{
			{ID: "comedy", Name: "Comedy"},
			{ID: "extra", Name: "Extra"},
		},
	}}}
	out := reconcileSettings(existing, true, cat)
	if !out.packOn("quips", "comedy") || out.packOn("quips", "extra") {
		t.Fatalf("new pack should default off: %#v", out.Enabled)
	}
}
