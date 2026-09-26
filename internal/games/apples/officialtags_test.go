package apples

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOfficialTagsFileParses(t *testing.T) {
	t.Parallel()
	if _, err := parseOfficialTags(officialTagsRaw); err != nil {
		t.Fatal(err)
	}
}

func TestParseOfficialTagsRejectsBadEntries(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"unknown tag": `{"formatVersion":1,"cards":{"0123456789abcdef":["gore"]}}`,
		"duplicate":   `{"formatVersion":1,"cards":{"0123456789abcdef":["sex","sex"]}}`,
		"kid mixed":   `{"formatVersion":1,"cards":{"0123456789abcdef":["kid friendly","violence"]}}`,
		"bad key":     `{"formatVersion":1,"cards":{"not-a-hash":["sex"]}}`,
		"version":     `{"formatVersion":2,"cards":{}}`,
	}
	for name, raw := range cases {
		if _, err := parseOfficialTags([]byte(raw)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestConvertOfficialAppliesOverlayTags(t *testing.T) {
	t.Parallel()
	raw := []byte(`[{"name":"CAH Base Set","official":true,
	  "white":[{"text":"A  Rubber chicken."},{"text":"Unreviewed card."}],
	  "black":[{"text":"My safe word is _.","pick":1}]}]`)
	tags := map[string][]string{
		officialTagKey("a rubber chicken."):  {"kid friendly"},
		officialTagKey("My safe word is _."): {"sex"},
	}
	lib, err := convertOfficialTagged(raw, tags)
	if err != nil {
		t.Fatal(err)
	}
	pack := lib.Packs[0]
	if got := pack.Prompts[0].Tags; len(got) != 1 || got[0] != "sex" {
		t.Fatalf("prompt tags = %v", got)
	}
	if got := pack.Answers[0].Tags; len(got) != 1 || got[0] != "kid friendly" {
		t.Fatalf("answer tags = %v", got)
	}
	if got := pack.Answers[1].Tags; got != nil {
		t.Fatalf("unreviewed card tags = %v", got)
	}
}

// TestOfficialTagCoverage reports official cards the overlay has not reviewed.
// Run it against a fresh upstream dump when JSON Against Humanity changes:
//
//	GRABBAG_CAH_DUMP=cah-all-full.json GRABBAG_CAH_MISSING=missing.jsonl \
//	  go test -run TestOfficialTagCoverage ./internal/games/apples/
//
// Each missing card is written as one JSON line with key, kind, pack, and text.
func TestOfficialTagCoverage(t *testing.T) {
	dump := os.Getenv("GRABBAG_CAH_DUMP")
	if dump == "" {
		t.Skip("set GRABBAG_CAH_DUMP to check overlay coverage")
	}
	raw, err := os.ReadFile(dump)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := parseOfficialTags(officialTagsRaw)
	if err != nil {
		t.Fatal(err)
	}
	lib, err := convertOfficialTagged(raw, tags)
	if err != nil {
		t.Fatal(err)
	}
	type missingCard struct {
		Key  string `json:"key"`
		Kind string `json:"kind"`
		Pack string `json:"pack"`
		Text string `json:"text"`
	}
	var missing []missingCard
	seen := map[string]bool{}
	total := 0
	note := func(kind, pack, text string) {
		key := officialTagKey(text)
		if seen[key] {
			return
		}
		seen[key] = true
		total++
		if _, ok := tags[key]; !ok {
			missing = append(missing, missingCard{Key: key, Kind: kind, Pack: pack, Text: text})
		}
	}
	for _, pack := range lib.Packs {
		for _, card := range pack.Prompts {
			note("prompt", pack.Name, card.Text)
		}
		for _, card := range pack.Answers {
			note("answer", pack.Name, card.Text)
		}
	}
	if out := os.Getenv("GRABBAG_CAH_MISSING"); out != "" {
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		enc := json.NewEncoder(f)
		enc.SetEscapeHTML(false)
		for _, card := range missing {
			if err := enc.Encode(card); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d of %d unique official cards reviewed", total-len(missing), total)
	if len(missing) > 0 {
		t.Fatalf("%d official cards missing from cahtags/official.json", len(missing))
	}
}
