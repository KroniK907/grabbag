package apples

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

//go:embed previews/*.json
var previewFiles embed.FS

var previewNow = time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)

// previewStage is one match moment the gallery can freeze.
type previewStage struct {
	name       string
	minBoard   int
	minJudge   int
	minSeated  int
	minWatcher int
}

var previewStages = []previewStage{
	{name: "draw", minBoard: 1, minJudge: 1, minSeated: 2, minWatcher: 1},
	{name: "hold", minBoard: 1, minJudge: 1, minSeated: 2, minWatcher: 1},
	{name: "choose", minBoard: 1, minJudge: 1, minSeated: 2, minWatcher: 1},
	{name: "submit", minBoard: 2, minJudge: 2, minSeated: 2, minWatcher: 2},
	{name: "submit-pick2", minBoard: 2, minJudge: 2, minSeated: 2, minWatcher: 2},
	{name: "submit-locked", minBoard: 3, minJudge: 3, minSeated: 3, minWatcher: 3},
	{name: "reveal", minBoard: 2, minJudge: 2, minSeated: 2, minWatcher: 2},
	{name: "favorites", minBoard: 2, minJudge: 2, minSeated: 2, minWatcher: 2},
	{name: "judge-pick", minBoard: 2, minJudge: 2, minSeated: 2, minWatcher: 2},
	{name: "sudden", minBoard: 2, minJudge: 3, minSeated: 2, minWatcher: 2},
	{name: "over", minBoard: 2, minJudge: 2, minSeated: 2, minWatcher: 2},
}

// Scenarios lists every Apples board, phone, and page state for the host
// -dev-preview gallery. Match states come from a real engine driven with
// fixed cards, names, seed, and clock.
func (g *Game) Scenarios() []ui.Scenario {
	var list []ui.Scenario
	for _, st := range previewStages {
		st := st
		list = append(list, ui.Scenario{
			Surface: "board", Group: st.name, Name: st.name, Viewer: "tv",
			Frame: ui.FrameTV, MinPlayers: st.minBoard, MaxPlayers: maxPlayers,
			Render: func(w io.Writer, p ui.Preview) error {
				pg, _, err := previewMatch(st.name, p.Players)
				if err != nil {
					return err
				}
				view := pg.boardViewLocked()
				view.pageView = previewPage(p, "Apples for Humanity")
				return ui.RenderScenario(w, pages, "board.html", view, p)
			},
		})
		for _, viewer := range []struct {
			id  string
			min int
		}{
			{"judge", st.minJudge},
			{"seated", st.minSeated},
			{"host", st.minSeated},
			{"audience", st.minWatcher},
		} {
			viewer := viewer
			list = append(list, ui.Scenario{
				Surface: "phone", Group: st.name, Name: st.name + "-" + viewer.id, Viewer: viewer.id,
				Frame: ui.FramePhone, Shell: ui.ShellPlayPhone, MinPlayers: viewer.min, MaxPlayers: maxPlayers,
				Sample: (st.name == "submit" && viewer.id == "seated") || (st.name == "reveal" && viewer.id == "judge"),
				Render: func(w io.Writer, p ui.Preview) error {
					pg, cast, err := previewMatch(st.name, p.Players)
					if err != nil {
						return err
					}
					who, err := cast.player(viewer.id)
					if err != nil {
						return err
					}
					view := pg.phoneViewLocked(who)
					view.pageView = previewPage(p, "Apples for Humanity")
					return ui.RenderScenario(w, pages, "phone.html", view, p)
				},
			})
		}
	}
	list = append(list,
		ui.Scenario{
			Surface: "phone", Name: "wait", Viewer: "seated", Frame: ui.FramePhone, Shell: ui.ShellPlayPhone,
			Render: func(w io.Writer, p ui.Preview) error {
				view := (&Game{}).phoneViewLocked(games.Player{ID: "p01", Seated: true})
				view.pageView = previewPage(p, "Apples for Humanity")
				return ui.RenderScenario(w, pages, "phone.html", view, p)
			},
		},
		ui.Scenario{
			Surface: "phone", Name: "howto-sheet", Viewer: "seated", Frame: ui.FramePhone, Shell: ui.ShellPlayPhone,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pages, "howto-sheet", previewHowto(p, true), p)
			},
		},
		ui.Scenario{
			Surface: "page", Name: "howto", Viewer: "guest", Frame: ui.FramePage,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pages, "howto.html", previewHowto(p, false), p)
			},
		},
		ui.Scenario{
			Surface: "page", Name: "picker", Viewer: "operator", Frame: ui.FramePage,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pages, "picker.html", previewPicker(p), p)
			},
		},
		ui.Scenario{
			Surface: "settings", Name: "match", Viewer: "operator", Frame: ui.FramePage, Shell: ui.ShellSettings,
			Render: func(w io.Writer, p ui.Preview) error {
				view := settingsView{
					pageView:        previewPage(p, "Apples for Humanity"),
					Settings:        factorySettings(),
					WildcardEnabled: true,
					DiscardEmpty:    true,
					Burns:           []burnEntry{{Kind: kindPrompt, Text: previewPrompts[0].text}, {Kind: kindAnswer, Text: previewAnswers[3]}},
				}
				return ui.RenderScenario(w, pages, "settings.html", view, p)
			},
		},
	)
	files, err := fs.Sub(previewFiles, "previews")
	if err != nil {
		panic("apples: embedded previews directory is missing")
	}
	out, err := ui.WithVariants(list, files)
	if err != nil {
		panic(err)
	}
	return out
}

func previewPage(p ui.Preview, title string) pageView {
	return pageView{
		Chrome:  ui.Chrome{Title: title, Theme: ui.NormalizeTheme(p.Theme)},
		GameCSS: p.Asset("game.css?v=" + assetVersion),
		GameJS:  p.Asset("game.js?v=" + assetVersion),
	}
}

func previewHowto(p ui.Preview, sheet bool) howtoView {
	return howtoView{
		pageView: previewPage(p, "How to play"), Sheet: sheet,
		Skip: true, Voting: true, Wildcard: true, Multiplier: true, Timers: true,
	}
}

func previewPicker(p ui.Preview) pickerView {
	view := pickerView{
		pageView: previewPage(p, "Deck Library"),
		DataDir:  "/home/host/.local/share/grabbag/games/apples",
		Tags: []tagView{
			{ID: "family", Label: "Family", Color: "#3aa675", Enabled: true},
			{ID: "spicy", Label: "Spicy", Color: "#e0463c"},
			{ID: "nerdy", Label: "Nerdy", Color: "#5a6cf0", Enabled: true},
		},
		Failed: []failedFile{{Filename: "broken-pack.json", Reason: "line 12: unexpected end of JSON input"}},
	}
	for li, lib := range []string{"Official", "House Rules", "Wildcards"} {
		lv := libraryView{
			ID: fmt.Sprintf("lib%d", li), Name: lib, Filename: fmt.Sprintf("lib%d.json", li),
			Description: "Cards for a Friday night with a long description that wraps onto a second line.",
			License:     "CC BY-NC-SA 4.0",
		}
		for pi := 0; pi < 3+li*2; pi++ {
			lv.Packs = append(lv.Packs, packView{
				LibraryID: lv.ID, ID: fmt.Sprintf("pack%d", pi), Name: fmt.Sprintf("%s Pack %d", lib, pi+1),
				Description: "A pack of prompts and answers.", Enabled: pi%2 == 0,
				PromptCount: 40 + pi*7, AnswerCount: 200 + pi*31,
				Dots: view.Tags[:1+pi%3],
			})
		}
		view.Libraries = append(view.Libraries, lv)
	}
	return view
}

// previewCast names who plays which part in a preview match.
type previewCast struct {
	judge  string
	humans []string
	names  map[string]string
}

func (c previewCast) player(viewer string) (games.Player, error) {
	switch viewer {
	case "judge":
		if c.judge == "" {
			return games.Player{}, fmt.Errorf("apples preview: no judge at this table size")
		}
		return games.Player{ID: c.judge, DisplayName: c.names[c.judge], Seated: true}, nil
	case "seated", "host":
		if len(c.humans) == 0 {
			return games.Player{}, fmt.Errorf("apples preview: no seated non-judge player at this table size")
		}
		id := c.humans[0]
		return games.Player{ID: id, DisplayName: c.names[id], Seated: true, ClaimedHost: viewer == "host"}, nil
	default:
		return games.Player{ID: "audience", DisplayName: "Audience"}, nil
	}
}

// previewMatch builds a Game whose engine sits at stage with n seated humans.
func previewMatch(stage string, n int) (*Game, previewCast, error) {
	if n < 1 {
		n = 1
	}
	settings := factorySettings()
	settings.PromptMode = modeSingle
	settings.AutoDrawSec = 15
	settings.BetweenRevealSec = 4
	settings.FinishHoldSec = 30
	switch stage {
	case "hold":
		settings.PromptMode = modeSkip
	case "choose":
		settings.PromptMode = modeMulti
	case "judge-pick":
		settings.FavoriteVoteSec = 0
	case "over":
		settings.FavoriteVoteSec = 0
		settings.WinByRounds = true
		settings.RoundLimit = 1
	}
	names := ui.PreviewNames(n)
	rows := make([]rosterRow, n)
	cast := previewCast{names: map[string]string{}}
	for i := range rows {
		rows[i] = rosterRow{ID: fmt.Sprintf("p%02d", i+1), Name: names[i]}
		cast.names[rows[i].ID] = names[i]
	}
	piles := previewPiles(n)
	e := &engine{}
	now := previewNow
	if out := e.Begin(settings, rows, piles, &previewSupply{piles: piles}, rand.New(rand.NewSource(7)), now); out.Err != nil || out.Shortage != "" {
		return nil, cast, fmt.Errorf("apples preview: begin: %v %s", out.Err, out.Shortage)
	}
	if stage == "submit-pick2" {
		e.Prompts = append([]playPrompt{previewPrompt(1, 2)}, e.Prompts...)
	}
	g := &Game{engine: e, now: func() time.Time { return previewNow }}
	do := func(cmd command) error {
		if out := e.Do(cmd, now); out.Err != nil {
			return fmt.Errorf("apples preview %s: %w", stage, out.Err)
		}
		return nil
	}
	humans := func() []string {
		var ids []string
		for _, row := range rows {
			if row.ID != e.JudgeID {
				ids = append(ids, row.ID)
			}
		}
		return ids
	}
	slotAndLock := func(id string, lock bool) error {
		a := e.Actors[id]
		pick := promptPick(e.LivePrompt)
		for i := 0; i < pick && len(a.Hand) > 0; i++ {
			if err := do(command{Kind: cmdSlot, Actor: id, Card: a.Hand[0].CardID}); err != nil {
				return err
			}
		}
		if lock {
			return do(command{Kind: cmdLock, Actor: id})
		}
		return nil
	}
	if stage == "sudden" {
		for i, row := range rows {
			e.Actors[row.ID].Score = 300 - 25*i
		}
		e.Actors[rows[0].ID].Score = 400
		if n > 1 {
			e.Actors[rows[1].ID].Score = 400
		}
		e.enterSudden()
		cast.judge, cast.humans = e.JudgeID, humans()
		return g, cast, nil
	}
	if stage != "draw" {
		if err := do(command{Kind: cmdDraw, Actor: e.JudgeID}); err != nil {
			return nil, cast, err
		}
	}
	cast.judge, cast.humans = e.JudgeID, humans()
	hs := cast.humans
	switch stage {
	case "submit", "submit-pick2":
		if len(hs) > 0 {
			if err := slotAndLock(hs[0], false); err != nil {
				return nil, cast, err
			}
		}
		for i := 1; i < len(hs); i += 2 {
			if err := slotAndLock(hs[i], true); err != nil {
				return nil, cast, err
			}
		}
	case "submit-locked":
		for _, id := range hs[:len(hs)-1] {
			if err := slotAndLock(id, true); err != nil {
				return nil, cast, err
			}
		}
	case "reveal", "favorites", "judge-pick", "over":
		for _, id := range hs {
			if err := slotAndLock(id, true); err != nil {
				return nil, cast, err
			}
		}
		reveals := len(e.Packets)
		if stage == "reveal" {
			reveals = (reveals + 1) / 2
		}
		for i := 0; i < reveals; i++ {
			if err := do(command{Kind: cmdReveal, Actor: e.JudgeID}); err != nil {
				return nil, cast, err
			}
		}
		previewVotes(e, hs)
		g.snapshotBurnDrawerLocked()
		if stage == "over" {
			for i, row := range rows {
				e.Actors[row.ID].Score = 5 * (i % 15)
			}
			if err := do(command{Kind: cmdConfirm, Actor: e.JudgeID, Card: e.Packets[0].ActorID}); err != nil {
				return nil, cast, err
			}
		}
	}
	return g, cast, nil
}

// previewVotes has every other human favorite the first face-up answer that
// is not their own.
func previewVotes(e *engine, voters []string) {
	for i, id := range voters {
		if i%2 == 1 {
			continue
		}
		for _, p := range e.Packets {
			if p.Revealed && p.ActorID != id {
				e.Do(command{Kind: cmdVote, Actor: id, Card: p.ActorID}, previewNow)
				break
			}
		}
	}
}

type previewSupply struct{ piles enabledPiles }

func (s *previewSupply) unplayed(matchSettings) enabledPiles { return s.piles }

func (s *previewSupply) recycleCovers(matchSettings, map[string]bool, int, int) bool { return true }

func (s *previewSupply) wildcardTexts() []string { return nil }

var previewPrompts = []struct {
	text string
	pick int
}{
	{"The new school lunch menu features _.", 1},
	{"What ruined the family reunion?", 1},
	{"My superpower is _, but only on Tuesdays.", 1},
	{"_ + _ = a surprisingly good weekend.", 2},
	{"In the director's cut, the hero finally confronts _ in an extremely long and dramatic monologue on a rooftop in the rain.", 1},
	{"What's in the box?", 1},
}

var previewAnswers = []string{
	"A llama in a tuxedo.",
	"Grandma's secret meatloaf.",
	"Tax fraud.",
	"An unreasonably confident squirrel.",
	"Screaming into the void and the void screaming back, politely, with a follow-up question.",
	"Glitter.",
	"The last slice of pizza.",
	"A strongly worded letter to the HOA.",
	"Interpretive dance.",
	"Forgetting the name of someone you just met.",
	"Supercalifragilisticexpialidocious.",
	"Dad jokes.",
	"A haunted Roomba.",
	"Crying in the IKEA parking lot.",
	"Emotional support crocodile.",
	"The entire discography of a kazoo cover band.",
	"Beans.",
	"Accidentally replying all.",
	"A suspiciously cheap timeshare.",
	"The Wi-Fi password.",
}

func previewPrompt(i, pick int) playPrompt {
	text := previewPrompts[i%len(previewPrompts)].text
	if pick == 2 {
		text = previewPrompts[3].text
	}
	return playPrompt{LibraryID: "preview", CardID: fmt.Sprintf("pp%d-%d", i, pick), Text: text, Pick: pick}
}

func previewPiles(n int) enabledPiles {
	var p enabledPiles
	for i := 0; i < 40; i++ {
		pick := previewPrompts[i%len(previewPrompts)].pick
		if pick != 1 {
			continue
		}
		p.Prompts = append(p.Prompts, playPrompt{
			LibraryID: "preview", CardID: fmt.Sprintf("p%d", i),
			Text: previewPrompts[i%len(previewPrompts)].text, Pick: 1,
		})
	}
	for i := 0; i < (n+4)*12; i++ {
		p.Answers = append(p.Answers, playCard{
			LibraryID: "preview", CardID: fmt.Sprintf("a%d", i),
			Text: previewAnswers[i%len(previewAnswers)],
		})
	}
	return p
}
