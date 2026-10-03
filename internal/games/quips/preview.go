package quips

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/games/runtimekit"
	"github.com/KroniK907/grabbag/internal/ui"
)

//go:embed previews/*.json
var previewFiles embed.FS

var previewNow = time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)

type previewStage struct {
	name      string
	minPlayer int
}

var previewStages = []previewStage{
	{name: "write", minPlayer: 2},
	{name: "write-locked", minPlayer: 3},
	{name: "parade-wait", minPlayer: 2},
	{name: "reveal", minPlayer: 2},
	{name: "vote", minPlayer: 2},
	{name: "hold", minPlayer: 2},
	{name: "last-quip", minPlayer: 2},
	{name: "final", minPlayer: 2},
}

// Scenarios lists every Quick Quips board, phone, and page state for the host
// -dev-preview gallery. Match states come from a real engine driven with
// fixed prompts, quips, names, seed, and clock.
func (g *Game) Scenarios() []ui.Scenario {
	var list []ui.Scenario
	for _, st := range previewStages {
		st := st
		list = append(list, ui.Scenario{
			Surface: "board", Group: st.name, Name: st.name, Viewer: "tv",
			Frame: ui.FrameTV, MinPlayers: st.minPlayer,
			Render: func(w io.Writer, p ui.Preview) error {
				pg, _, err := previewMatch(st.name, p.Players)
				if err != nil {
					return err
				}
				view := pg.boardViewLocked()
				view.Page = previewPage(p, "Quick Quips")
				return ui.RenderScenario(w, pages, "board.html", view, p)
			},
		})
		for _, viewer := range []string{"seated", "host", "audience"} {
			viewer := viewer
			list = append(list, ui.Scenario{
				Surface: "phone", Group: st.name, Name: st.name + "-" + viewer, Viewer: viewer,
				Frame: ui.FramePhone, Shell: ui.ShellPlayPhone, MinPlayers: st.minPlayer,
				Sample: viewer == "seated" && (st.name == "write" || st.name == "vote"),
				Render: func(w io.Writer, p ui.Preview) error {
					pg, cast, err := previewMatch(st.name, p.Players)
					if err != nil {
						return err
					}
					view := pg.phoneViewLocked(cast.player(viewer))
					view.Page = previewPage(p, "Quick Quips")
					return ui.RenderScenario(w, pages, "phone.html", view, p)
				},
			})
		}
	}
	list = append(list,
		ui.Scenario{
			Surface: "phone", Name: "wait", Viewer: "seated", Frame: ui.FramePhone, Shell: ui.ShellPlayPhone,
			Render: func(w io.Writer, p ui.Preview) error {
				view := New().phoneViewLocked(games.Player{ID: "p01", Seated: true})
				view.Page = previewPage(p, "Quick Quips")
				return ui.RenderScenario(w, pages, "phone.html", view, p)
			},
		},
		ui.Scenario{
			Surface: "page", Name: "howto", Viewer: "guest", Frame: ui.FramePage,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pages, "howto.html", previewPage(p, "How to play Quick Quips"), p)
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
					Page:         previewPage(p, "Quick Quips settings"),
					Settings:     factorySettings(),
					DiscardEmpty: true,
					Burns:        []burnEntry{{Kind: kindPrompt, Text: previewPromptTexts[0]}},
				}
				return ui.RenderScenario(w, pages, "settings.html", view, p)
			},
		},
	)
	files, err := fs.Sub(previewFiles, "previews")
	if err != nil {
		panic("quips: embedded previews directory is missing")
	}
	out, err := ui.WithVariants(list, files)
	if err != nil {
		panic(err)
	}
	return out
}

func previewPage(p ui.Preview, title string) runtimekit.Page {
	return runtimekit.Page{
		Chrome:  ui.Chrome{Title: title, Theme: ui.NormalizeTheme(p.Theme)},
		GameCSS: p.Asset("game.css?v=" + assetVersion),
		GameJS:  p.Asset("game.js?v=" + assetVersion),
	}
}

func previewPicker(p ui.Preview) pickerView {
	view := pickerView{
		Page:    previewPage(p, "Prompt Library"),
		DataDir: "/home/host/.local/share/grabbag/games/quips",
		Failed:  []failedFile{{Filename: "broken-pack.json", Reason: "line 3: prompts must be a list"}},
	}
	for li, lib := range []string{"Official", "House Prompts"} {
		lv := libraryView{
			ID: fmt.Sprintf("lib%d", li), Name: lib, Filename: fmt.Sprintf("lib%d.json", li),
			Description: "Prompts for a room that likes to argue about the answers.",
			License:     "CC BY 4.0",
		}
		for pi := 0; pi < 3+li*3; pi++ {
			lv.Packs = append(lv.Packs, packView{
				LibraryID: lv.ID, ID: fmt.Sprintf("pack%d", pi), Name: fmt.Sprintf("%s Pack %d", lib, pi+1),
				Description: "A pack of prompts.", Enabled: pi%2 == 0, PromptCount: 60 + pi*13,
			})
		}
		view.Libraries = append(view.Libraries, lv)
	}
	return view
}

type previewCast struct {
	seated string
	names  map[string]string
}

func (c previewCast) player(viewer string) games.Player {
	switch viewer {
	case "seated", "host":
		return games.Player{ID: c.seated, DisplayName: c.names[c.seated], Seated: true, ClaimedHost: viewer == "host"}
	default:
		return games.Player{ID: "audience", DisplayName: "Audience"}
	}
}

// previewMatch builds a Game whose engine sits at stage with n seated writers.
func previewMatch(stage string, n int) (*Game, previewCast, error) {
	if n < 2 {
		n = 2
	}
	settings := factorySettings()
	settings.LiveVoteCounts = true
	switch stage {
	case "parade-wait", "reveal":
		settings.HostControlledReveals = true
	case "last-quip":
		settings.RoundCount = 1
	}
	if stage != "last-quip" {
		settings.LastQuipEnabled = false
	}
	names := ui.PreviewNames(n)
	rows := make([]rosterRow, n)
	cast := previewCast{seated: "p01", names: map[string]string{}}
	for i := range rows {
		rows[i] = rosterRow{ID: fmt.Sprintf("p%02d", i+1), Name: names[i]}
		cast.names[rows[i].ID] = names[i]
	}
	pool := make([]playPrompt, 0, n*2)
	for i := 0; i < n*2; i++ {
		pool = append(pool, playPrompt{LibraryID: "preview", CardID: fmt.Sprintf("q%d", i), Text: previewPromptTexts[i%len(previewPromptTexts)]})
	}
	clock := func() time.Time { return previewNow }
	e := &engine{now: clock}
	if err := e.Begin(settings, rows, promptDeal{Pool: pool}, EngineHooks{}, rand.New(rand.NewSource(7)), previewNow); err != nil {
		return nil, cast, fmt.Errorf("quips preview: begin: %w", err)
	}
	if e.Timer.Kind == timerPlayIntro || e.Timer.Kind == timerLastIntro {
		e.clearTimer()
		if e.Settings.WriteSec > 0 {
			e.armTimer(previewNow, "write", e.Settings.WriteSec)
		}
	}
	g := fixedGame(e, clock)
	g.burnDrawer = []burnFace{
		{Kind: kindPrompt, Text: previewPromptTexts[1]},
		{Kind: kindPrompt, Text: previewPromptTexts[2]},
	}
	lock := func(id string) error {
		w := e.Writers[id]
		var msg string
		for _, tagged := range []bool{false, true} {
			drafts := make([]string, len(w.Slots))
			for i := range drafts {
				drafts[i] = previewQuip(id, i, tagged)
			}
			out := e.Do(Command{Kind: CmdLock, Actor: id, Drafts: drafts}, previewNow)
			if msg = out.PhoneErr[id]; msg == "" {
				return nil
			}
		}
		return fmt.Errorf("quips preview %s: lock %s: %s", stage, id, msg)
	}
	switch stage {
	case "write":
		e.Do(Command{Kind: CmdDraft, Actor: "p01", Slot: 0, Text: previewQuip("p01", 0, false)}, previewNow)
		for i := 2; i < n; i += 2 {
			if err := lock(rows[i].ID); err != nil {
				return nil, cast, err
			}
		}
		return g, cast, nil
	case "write-locked":
		for _, row := range rows[:n-1] {
			if err := lock(row.ID); err != nil {
				return nil, cast, err
			}
		}
		return g, cast, nil
	}
	for _, row := range rows {
		if err := lock(row.ID); err != nil {
			return nil, cast, err
		}
	}
	switch stage {
	case "reveal":
		e.Do(Command{Kind: CmdHostNextSegment, Actor: "host"}, previewNow)
		e.Do(Command{Kind: CmdHostReveal, Actor: "host"}, previewNow)
	case "vote", "hold", "last-quip":
		cast.seated = previewVoter(e, rows)
		previewVotes(e, rows)
		if stage == "hold" {
			e.closeVote(previewNow, Outcome{})
		}
	case "final":
		for i, row := range rows {
			e.Scores[row.ID] = 100 * ((n - i) % 7)
		}
		e.enterFinalScores(previewNow, Outcome{})
	}
	return g, cast, nil
}

// previewVoter picks a seated player who is not writing the live segment,
// so the phone shows vote options instead of "your quip is up".
func previewVoter(e *engine, rows []rosterRow) string {
	busy := map[string]bool{}
	for _, id := range e.segmentWriters(e.activeSegmentIdx()) {
		busy[id] = true
	}
	for _, row := range rows {
		if !busy[row.ID] {
			return row.ID
		}
	}
	return rows[0].ID
}

func previewVotes(e *engine, rows []rosterRow) {
	for i, row := range rows {
		if i%3 == 0 {
			continue
		}
		targets := e.voteTargets(row.ID)
		if len(targets) == 0 {
			continue
		}
		e.Do(Command{Kind: CmdVote, Actor: row.ID, VoteTarget: targets[i%len(targets)], VoterSeat: true}, previewNow)
	}
}

var previewPromptTexts = []string{
	"The worst thing to hear from your pilot",
	"A rejected flavor of ice cream",
	"What the cat is actually thinking",
	"The title of your autobiography",
	"A terrible name for a boat, and the story of how the boat got that name after a long night",
	"Something you should never say at a wedding",
}

var previewQuipTexts = []string{
	"Oops.",
	"Pickle-flavored regret",
	"I have calculated every possible outcome and you lose.",
	"Mostly Harmless: The Unauthorized Biography of a Man Who Owns Seventeen Kazoos",
	"We'll be landing shortly, somewhere",
	"Beans",
	"Supercalifragilisticexpialidocious",
	"Mayonnaise surprise",
}

// previewQuip is a fixed quip for a writer's slot. tagged adds the writer id
// so two writers on one prompt never lock the same text.
func previewQuip(id string, slot int, tagged bool) string {
	var n int
	fmt.Sscanf(id, "p%d", &n)
	text := previewQuipTexts[(n*3+slot)%len(previewQuipTexts)]
	if tagged {
		text += " #" + id
	}
	return text
}
