package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path"
	"reflect"
	"sort"
	"strings"
)

// Preview frames name the reference viewport a scenario is drawn for.
const (
	// FramePhone is a phone column (Lobby room, game phone bodies).
	FramePhone = "phone"
	// FrameTV is the full-bleed /board document.
	FrameTV = "tv"
	// FramePage is an operator page such as /settings.
	FramePage = "page"
)

// ShellBoard marks a scenario whose Render writes a tenant's board fragment.
// Host wraps it in the static board shell with the tenant's CSS.
const ShellBoard = "board"

// ShellPhone marks a scenario whose Render writes a tenant's whole phone
// fragment (the Lobby join and room screens). Host wraps it in the static
// phone shell.
const ShellPhone = "phone"

// ShellPlayPhone marks a scenario whose Render writes a game phone body.
// Host wraps it in the Lobby play-phone fragment, then in the static phone
// shell, as it does for live phones. A "host" Viewer also gets the Host panel
// in the frame.
const ShellPlayPhone = "play-phone"

// ShellHelp marks a scenario whose Render writes a HelpTenant's Help body.
// Host shows it in the static phone shell's open Help sheet.
const ShellHelp = "help"

// ShellSettings marks a scenario whose Render writes a game settings
// fragment. Host inlines it under Game Settings on the Lobby /settings page.
const ShellSettings = "settings"

// Player-count sweep bounds for previews.
const (
	// LobbyMaxPlayers is the Lobby seat cap ceiling. Scenarios with no
	// declared max sweep up to it as a stress case.
	LobbyMaxPlayers = 64
	// RealisticMaxPlayers is the largest table previews treat as normal play.
	RealisticMaxPlayers = 12
)

// Scenario is one named UI state a package renders from fixed data, without
// running its logic. Packages return these from a Scenarios func. Host serves
// them at /dev/ui when started with -dev-preview.
type Scenario struct {
	// Surface is the page family, such as "board", "phone", "join", "settings".
	Surface string
	// Group ties the surfaces of one moment together for the table view,
	// such as "reveal". Empty means the scenario only shows alone.
	Group string
	// Name is unique within Surface. Lowercase, digits, and dashes.
	Name string
	// Viewer is who is looking: "tv", "judge", "seated", "audience", "host", "guest", "operator".
	Viewer string
	// Frame is FramePhone, FrameTV, or FramePage.
	Frame string
	// Shell is empty for a full document, or ShellBoard, ShellPhone,
	// ShellPlayPhone, ShellHelp, or ShellSettings.
	Shell string
	// HostPanel writes the claimed host's panel for the phone frame, when
	// this moment has one. Nil means none, except that host adds the Lobby's
	// panel to ShellPlayPhone scenarios whose Viewer is "host".
	HostPanel func(w io.Writer, p Preview) error
	// MinPlayers > 0 turns on the seated-count sweep. MaxPlayers 0 means the
	// package declares no cap and the sweep goes to LobbyMaxPlayers.
	MinPlayers int
	MaxPlayers int
	// Sample puts the scenario in the device and text-size matrix.
	Sample bool
	// Open is a space-separated list of element ids the static shell opens
	// when the preview request names none, such as a drawer.
	Open string
	// Patches are saved JSON merge patches applied before the request patch.
	Patches [][]byte
	// Render writes the page. It must not touch disk, the network, or clocks.
	Render func(w io.Writer, p Preview) error
}

// Preview is the per-request input to Scenario.Render.
type Preview struct {
	// Theme is a ui theme id. Empty means DefaultTheme.
	Theme string
	// Players is the seated count to build the state with. 0 means the
	// scenario default.
	Players int
	// Patches are JSON merge patches (RFC 7386) applied to the view struct,
	// in order, just before the template runs.
	Patches [][]byte
	// Assets is the URL prefix for the package's static/ files, ending in "/".
	Assets string
	// Open is a space-separated list of element ids to open on load. Empty
	// keeps whatever the scenario opened itself.
	Open string
}

// Asset returns the URL for a file in the package's static/ directory.
func (p Preview) Asset(name string) string {
	return p.Assets + name
}

// Sweep returns the seated counts a scenario is previewed at: min, a middle
// table, RealisticMaxPlayers, and the declared max (or LobbyMaxPlayers).
// It returns nil when the scenario has no player dimension.
func (s Scenario) Sweep() []int {
	if s.MinPlayers <= 0 {
		return nil
	}
	max := s.MaxPlayers
	if max <= 0 {
		max = LobbyMaxPlayers
	}
	realistic := min(RealisticMaxPlayers, max)
	counts := map[int]bool{s.MinPlayers: true, realistic: true, max: true}
	counts[(s.MinPlayers+realistic+1)/2] = true
	out := make([]int, 0, len(counts))
	for n := range counts {
		if n >= s.MinPlayers && n <= max {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

// DefaultPlayers is the middle of the realistic range, or 0 without a sweep.
func (s Scenario) DefaultPlayers() int {
	if s.MinPlayers <= 0 {
		return 0
	}
	max := s.MaxPlayers
	if max <= 0 {
		max = LobbyMaxPlayers
	}
	return (s.MinPlayers + min(RealisticMaxPlayers, max) + 1) / 2
}

// RealisticPlayers is the largest normal table for the scenario, or 0.
func (s Scenario) RealisticPlayers() int {
	if s.MinPlayers <= 0 {
		return 0
	}
	if s.MaxPlayers > 0 {
		return min(RealisticMaxPlayers, s.MaxPlayers)
	}
	return RealisticMaxPlayers
}

// ClampPlayers maps a requested count into the scenario's range.
func (s Scenario) ClampPlayers(n int) int {
	if s.MinPlayers <= 0 {
		return 0
	}
	if n <= 0 {
		return s.DefaultPlayers()
	}
	hi := s.MaxPlayers
	if hi <= 0 {
		hi = LobbyMaxPlayers
	}
	return min(max(s.MinPlayers, n), hi)
}

// RenderScenario runs template name with view after stamping preview chrome
// (Static, Theme, Open) and applying p.Patches. view must be a struct value
// whose exported fields round-trip through encoding/json. Views that embed
// Chrome get the stamp; other views only get the patches.
func RenderScenario(w io.Writer, t *template.Template, name string, view any, p Preview) error {
	out, err := PatchView(view, p)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, out); err != nil {
		return err
	}
	_, err = buf.WriteTo(w)
	return err
}

// PatchView returns a copy of view with preview chrome and p.Patches applied.
func PatchView(view any, p Preview) (any, error) {
	raw, err := json.Marshal(view)
	if err != nil {
		return nil, fmt.Errorf("ui: preview marshal: %w", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("ui: preview view must be a struct: %w", err)
	}
	if _, ok := doc["Static"]; ok {
		doc["Static"] = true
		doc["Theme"] = NormalizeTheme(p.Theme)
		if p.Open != "" {
			doc["OpenIDs"] = p.Open
		}
	}
	for i, patch := range p.Patches {
		if len(bytes.TrimSpace(patch)) == 0 {
			continue
		}
		var mp any
		if err := json.Unmarshal(patch, &mp); err != nil {
			return nil, fmt.Errorf("ui: preview patch %d: %w", i, err)
		}
		merged, ok := MergePatch(doc, mp).(map[string]any)
		if !ok {
			return nil, fmt.Errorf("ui: preview patch %d must be a JSON object", i)
		}
		doc = merged
	}
	raw, err = json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	ptr := reflect.New(reflect.TypeOf(view))
	if err := json.Unmarshal(raw, ptr.Interface()); err != nil {
		return nil, fmt.Errorf("ui: preview patch does not fit the view: %w", err)
	}
	return ptr.Elem().Interface(), nil
}

// MergePatch applies an RFC 7386 JSON merge patch to target and returns the
// result. Objects merge by key, null deletes a key, anything else replaces.
func MergePatch(target, patch any) any {
	pm, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	tm, ok := target.(map[string]any)
	if !ok {
		tm = map[string]any{}
	}
	out := make(map[string]any, len(tm))
	for k, v := range tm {
		out[k] = v
	}
	for k, v := range pm {
		if v == nil {
			delete(out, k)
			continue
		}
		out[k] = MergePatch(out[k], v)
	}
	return out
}

// WithVariants adds one scenario per saved patch file in files. A file named
// "<surface>.<name>.<variant>.json" copies that scenario as
// "<name>~<variant>" with the file as its first patch. Unmatched files are
// returned as an error so a renamed scenario does not drop variants silently.
func WithVariants(list []Scenario, files fs.FS) ([]Scenario, error) {
	if files == nil {
		return list, nil
	}
	names, err := fs.Glob(files, "*.json")
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	for i, s := range list {
		index[s.Surface+"."+s.Name] = i
	}
	out := append([]Scenario(nil), list...)
	for _, file := range names {
		base := strings.TrimSuffix(path.Base(file), ".json")
		dot := strings.LastIndex(base, ".")
		if dot < 0 {
			return nil, fmt.Errorf("ui: preview variant %s: want <surface>.<name>.<variant>.json", file)
		}
		key, variant := base[:dot], base[dot+1:]
		i, ok := index[key]
		if !ok {
			return nil, fmt.Errorf("ui: preview variant %s: no scenario %s", file, key)
		}
		raw, err := fs.ReadFile(files, file)
		if err != nil {
			return nil, err
		}
		if !json.Valid(raw) {
			return nil, fmt.Errorf("ui: preview variant %s is not JSON", file)
		}
		v := list[i]
		v.Name = v.Name + "~" + variant
		v.Patches = append([][]byte{raw}, v.Patches...)
		out = append(out, v)
	}
	return out, nil
}

var previewNames = []string{
	"Sam", "Alexandria-Rose", "Bo", "Zoë", "Maximilian", "Priya", "Ji-woo",
	"DJ Longname III", "Kat", "Oluwaseun", "Mo", "Guinevere Blackwood",
	"Ana", "Theodora", "Li Wei", "Björn", "Xochitl", "Ed", "Marguerite",
	"Kai",
}

// PreviewNames returns n distinct display names with a deliberate mix of
// short, long, and non-ASCII names. It is the same list for the same n.
func PreviewNames(n int) []string {
	out := make([]string, n)
	for i := range out {
		name := previewNames[i%len(previewNames)]
		if round := i / len(previewNames); round > 0 {
			name = fmt.Sprintf("%s %d", name, round+1)
		}
		out[i] = name
	}
	return out
}
