package borrowedtruths

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
)

const kvMatchSettings = "match-settings"

// Lie sources.
const (
	lieSourcePlayers = "players"
	lieSourceBank    = "bank"
	lieSourceMixed   = "mixed"
)

// This Is My placements.
const (
	placeMiddle    = "middle"
	placeAlternate = "alternate"
)

// timAuto is the This Is My rounds setting that follows the seated count.
const timAuto = "auto"

// I knew it void shares.
const (
	voidThird     = "third"
	voidHalf      = "half"
	voidTwoThirds = "two-thirds"
)

// matchSettings is the game KV document at match-settings. Streamer mode is
// not in the first playable.
type matchSettings struct {
	Timers          bool   `json:"timers"`
	TruthsPerPlayer int    `json:"truthsPerPlayer"`
	LiesPerPlayer   int    `json:"liesPerPlayer"`
	LieSource       string `json:"lieSource"`
	MixYours        int    `json:"mixYours"`
	MixBorrowed     int    `json:"mixBorrowed"`
	MixLie          int    `json:"mixLie"`
	TellsPerPlayer  int    `json:"tellsPerPlayer"` // 0 is Auto
	MaxTellsLarge   int    `json:"maxTellsLarge"`
	FactsSec        int    `json:"factsSec"`
	PrivateSec      int    `json:"privateSec"`
	QuestionSec     int    `json:"questionSec"`
	VoteSec         int    `json:"voteSec"`
	OwnerVoteSec    int    `json:"ownerVoteSec"`
	KnewIt          bool   `json:"knewIt"`
	KnewVoid        string `json:"knewVoid"`
	Skips           int    `json:"skips"`
	AudienceVote    bool   `json:"audienceVote"`
	TIMRounds       string `json:"timRounds"` // timAuto or "0" to "12"
	TIMPlacement    string `json:"timPlacement"`
	NotTheirsPct    int    `json:"notTheirsPct"`
	LookSec         int    `json:"lookSec"`
	ClaimSec        int    `json:"claimSec"`
	TIMQuestionSec  int    `json:"timQuestionSec"`
}

func factorySettings() matchSettings {
	return matchSettings{
		TruthsPerPlayer: 2,
		LiesPerPlayer:   1,
		LieSource:       lieSourceMixed,
		MixYours:        1,
		MixBorrowed:     1,
		MixLie:          1,
		MaxTellsLarge:   12,
		FactsSec:        180,
		PrivateSec:      30,
		QuestionSec:     90,
		VoteSec:         20,
		OwnerVoteSec:    15,
		KnewIt:          true,
		KnewVoid:        voidHalf,
		Skips:           1,
		AudienceVote:    true,
		TIMRounds:       timAuto,
		TIMPlacement:    placeMiddle,
		NotTheirsPct:    15,
		LookSec:         20,
		ClaimSec:        30,
		TIMQuestionSec:  180,
	}
}

// timRounds is the This Is My round count before the photo and placement
// caps. Auto is 0 below 5 seated, 1 at 5 to 7, 2 at 8 to 12, 3 at 13 and up.
func (s matchSettings) timRounds(seated int) int {
	if n, err := strconv.Atoi(s.TIMRounds); err == nil {
		return n
	}
	switch {
	case seated < 5:
		return 0
	case seated <= 7:
		return 1
	case seated <= 12:
		return 2
	}
	return 3
}

// lieSource is the lie source in play. Zero lies per player means bank only,
// whatever the form says, so a Lie weight always has cards to deal.
func (s matchSettings) lieSource() string {
	if s.LiesPerPlayer == 0 {
		return lieSourceBank
	}
	return s.LieSource
}

// voidShare is the I knew it void threshold as num/den.
func (s matchSettings) voidShare() (num, den int) {
	switch s.KnewVoid {
	case voidThird:
		return 1, 3
	case voidTwoThirds:
		return 2, 3
	default:
		return 1, 2
	}
}

type settingsErr struct {
	Field string
	Msg   string
}

// field is one validated integer knob on the settings form.
type field struct {
	name   string
	label  string
	lo, hi int
	dst    *int
}

// applyForm copies every settings form value into s. The first bad value
// stops the write and names the field.
func applyForm(r *http.Request, s *matchSettings) *settingsErr {
	s.Timers = r.FormValue("timers") == "1"
	s.KnewIt = r.FormValue("knew_it") == "1"
	s.AudienceVote = r.FormValue("audience_vote") == "1"
	switch v := r.FormValue("lie_source"); v {
	case lieSourcePlayers, lieSourceBank, lieSourceMixed:
		s.LieSource = v
	default:
		return &settingsErr{"lie_source", "Pick a lie source."}
	}
	switch v := r.FormValue("tim_rounds"); v {
	case timAuto:
		s.TIMRounds = v
	default:
		if n, err := strconv.Atoi(v); err != nil || n < 0 || n > 12 {
			return &settingsErr{"tim_rounds", "This Is My rounds is Auto or 0 to 12."}
		}
		s.TIMRounds = v
	}
	switch v := r.FormValue("tim_placement"); v {
	case placeMiddle, placeAlternate:
		s.TIMPlacement = v
	default:
		return &settingsErr{"tim_placement", "Pick a This Is My placement."}
	}
	switch v := r.FormValue("knew_void"); v {
	case voidThird, voidHalf, voidTwoThirds:
		s.KnewVoid = v
	default:
		return &settingsErr{"knew_void", "Pick a void share."}
	}
	for _, f := range numFields(s) {
		n, err := strconv.Atoi(r.FormValue(f.name))
		if err != nil || n < f.lo || n > f.hi {
			return &settingsErr{f.name, fmt.Sprintf("%s is %d to %d.", f.label, f.lo, f.hi)}
		}
		*f.dst = n
	}
	if s.MixYours+s.MixBorrowed+s.MixLie == 0 {
		return &settingsErr{"mix_yours", "At least one card type needs a weight."}
	}
	return nil
}

// numFields are the integer knobs on the settings form, in form order.
func numFields(s *matchSettings) []field {
	return []field{
		{"truths", "Truths per player", 1, 3, &s.TruthsPerPlayer},
		{"lies", "Lies per player", 0, 2, &s.LiesPerPlayer},
		{"mix_yours", "Yours weight", 0, 3, &s.MixYours},
		{"mix_borrowed", "Borrowed weight", 0, 3, &s.MixBorrowed},
		{"mix_lie", "Lie weight", 0, 3, &s.MixLie},
		{"tells", "Tells per player", 0, 3, &s.TellsPerPlayer},
		{"max_tells", "Max tells above 12 seated", 8, 20, &s.MaxTellsLarge},
		{"facts_sec", "Facts timer", 60, 300, &s.FactsSec},
		{"private_sec", "Private read timer", 15, 60, &s.PrivateSec},
		{"question_sec", "Questioning timer", 45, 300, &s.QuestionSec},
		{"vote_sec", "Vote timer", 10, 45, &s.VoteSec},
		{"owner_vote_sec", "Owner vote timer", 10, 30, &s.OwnerVoteSec},
		{"skips", "Teller skips per turn", 0, 2, &s.Skips},
		{"not_theirs", "Not theirs chance", 0, 33, &s.NotTheirsPct},
		{"look_sec", "Look time", 15, 30, &s.LookSec},
		{"claim_sec", "Claim timer", 15, 60, &s.ClaimSec},
		{"tim_question_sec", "This Is My questioning timer", 120, 300, &s.TIMQuestionSec},
	}
}

func (g *Game) loadSettings(h games.Helper) matchSettings {
	s := factorySettings()
	raw, found, err := h.KVGet(kvMatchSettings)
	if err != nil || !found {
		return s
	}
	if json.Unmarshal(raw, &s) != nil {
		return factorySettings()
	}
	return s
}

func (g *Game) saveSettings(h games.Helper, s matchSettings) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return h.KVSet(kvMatchSettings, raw)
}

type settingsView struct {
	runtimekit.Page
	Frozen   bool
	Settings matchSettings
	Err      settingsErr
	Nums     map[string]numView
	// TIMCounts are the fixed This Is My round choices after Auto.
	TIMCounts []string
}

// numView is one number input and its error line.
type numView struct {
	Name, Label string
	Lo, Hi      int
	Value       int
	Err         string
}

func newSettingsView(page runtimekit.Page, s matchSettings, e settingsErr, frozen bool) settingsView {
	view := settingsView{Page: page, Frozen: frozen, Settings: s, Err: e, Nums: map[string]numView{}}
	for i := 0; i <= 12; i++ {
		view.TIMCounts = append(view.TIMCounts, strconv.Itoa(i))
	}
	for _, f := range numFields(&s) {
		n := numView{Name: f.name, Label: f.label, Lo: f.lo, Hi: f.hi, Value: *f.dst}
		if e.Field == f.name {
			n.Err = e.Msg
		}
		view.Nums[f.name] = n
	}
	return view
}

func (g *Game) settingsView(e settingsErr) settingsView {
	s := factorySettings()
	if h := g.run.Helper(); h != nil {
		s = g.loadSettings(h)
	}
	g.run.Lock()
	frozen := g.run.Running()
	g.run.Unlock()
	return newSettingsView(g.run.Page("Borrowed Truths settings"), s, e, frozen)
}

func (g *Game) getSettings(w http.ResponseWriter, r *http.Request) {
	g.run.Render(w, "settings.html", g.settingsView(settingsErr{}), http.StatusOK)
}

func (g *Game) postMatch(w http.ResponseWriter, r *http.Request) {
	h := g.run.Helper()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	g.run.Lock()
	frozen := g.run.Running()
	g.run.Unlock()
	if frozen {
		g.run.Render(w, "settings.html", g.settingsView(settingsErr{"", "This match is running. Setup is locked."}), http.StatusOK)
		return
	}
	s := g.loadSettings(h)
	if bad := applyForm(r, &s); bad != nil {
		g.run.Render(w, "settings.html", g.settingsView(*bad), http.StatusOK)
		return
	}
	if err := g.saveSettings(h, s); err != nil {
		g.run.Render(w, "settings.html", g.settingsView(settingsErr{"", "Could not save the settings."}), http.StatusOK)
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		g.run.Render(w, "settings.html", g.settingsView(settingsErr{}), http.StatusOK)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}
