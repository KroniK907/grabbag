package borrowedtruths

import (
	"fmt"
	"html/template"
	"io"
	"math/rand"
	"net/url"
	"time"

	"github.com/KroniK907/grabbag/internal/games"
	"github.com/KroniK907/grabbag/internal/ui"
)

var previewNow = time.Date(2026, 9, 27, 20, 0, 0, 0, time.UTC)

// previewStages are the moments of one tell, in play order. kind forces the
// dealt card type so the borrowed path and a player lie both show.
var previewStages = []struct {
	name string
	kind cardKind
}{
	{"facts", kindBorrowed},
	{"private-read", kindBorrowed},
	{"public-read", kindBorrowed},
	{"vote", kindBorrowed},
	{"vote-closed", kindBorrowed},
	{"reveal", kindBorrowed},
	{"reveal-lie", kindLie},
	{"owner-vote", kindBorrowed},
	{"owner-reveal", kindBorrowed},
	{"standings", kindBorrowed},
	{"final", kindBorrowed},
}

// previewViewers are the phones drawn for each moment. teller and insider
// are seated phones with private lines.
var previewViewers = []struct {
	name, viewer string
}{
	{"teller", "seated"},
	{"insider", "seated"},
	{"voter", "seated"},
	{"host", "host"},
	{"audience", "audience"},
}

// Scenarios lists every Borrowed Truths board, phone, and page state for the
// host -dev-preview gallery. Match states come from a real engine driven with
// fixed facts, names, seed, and clock.
func (g *Game) Scenarios() []ui.Scenario {
	var list []ui.Scenario
	for _, st := range previewStages {
		st := st
		list = append(list, ui.Scenario{
			Surface: "board", Group: st.name, Name: st.name, Viewer: "tv",
			Frame: ui.FrameTV, MinPlayers: 4, MaxPlayers: 20,
			Render: func(w io.Writer, p ui.Preview) error {
				pg, _, err := previewMatch(st.name, st.kind, p.Players)
				if err != nil {
					return err
				}
				view := pg.boardViewLocked()
				view.pageView = previewPage(p, "Borrowed Truths")
				return ui.RenderScenario(w, pages, "board.html", view, p)
			},
		})
		for _, v := range previewViewers {
			v := v
			list = append(list, ui.Scenario{
				Surface: "phone", Group: st.name, Name: st.name + "-" + v.name, Viewer: v.viewer,
				Frame: ui.FramePhone, Shell: ui.ShellPlayPhone, MinPlayers: 4, MaxPlayers: 20,
				Sample: (st.name == "vote" && v.name == "voter") || (st.name == "private-read" && v.name == "teller"),
				Render: func(w io.Writer, p ui.Preview) error {
					pg, cast, err := previewMatch(st.name, st.kind, p.Players)
					if err != nil {
						return err
					}
					view := pg.phoneViewFor(cast[v.name], previewNow)
					view.pageView = previewPage(p, "Borrowed Truths")
					return ui.RenderScenario(w, pages, "phone.html", view, p)
				},
			})
		}
	}
	for _, st := range previewTIMStages {
		st := st
		list = append(list, ui.Scenario{
			Surface: "board", Group: st, Name: st, Viewer: "tv",
			Frame: ui.FrameTV, MinPlayers: 5, MaxPlayers: 20,
			Render: func(w io.Writer, p ui.Preview) error {
				pg, _, err := previewTIM(st, p.Players)
				if err != nil {
					return err
				}
				view := pg.boardViewLocked()
				view.pageView = previewPage(p, "Borrowed Truths")
				return ui.RenderScenario(w, pages, "board.html", view, p)
			},
		})
		for _, v := range previewTIMViewers {
			v := v
			list = append(list, ui.Scenario{
				Surface: "phone", Group: st, Name: st + "-" + v.name, Viewer: v.viewer,
				Frame: ui.FramePhone, Shell: ui.ShellPlayPhone, MinPlayers: 5, MaxPlayers: 20,
				Sample: st == "tim-vote" && v.name == "voter",
				Render: func(w io.Writer, p ui.Preview) error {
					pg, cast, err := previewTIM(st, p.Players)
					if err != nil {
						return err
					}
					view := pg.phoneViewFor(cast[v.name], previewNow)
					view.pageView = previewPage(p, "Borrowed Truths")
					return ui.RenderScenario(w, pages, "phone.html", view, p)
				},
			})
		}
	}
	list = append(list,
		ui.Scenario{
			Surface: "page", Name: "howto", Viewer: "guest", Frame: ui.FramePage,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pages, "howto.html", previewPage(p, "How to play Borrowed Truths"), p)
			},
		},
		ui.Scenario{
			Surface: "settings", Name: "match", Viewer: "operator", Frame: ui.FramePage, Shell: ui.ShellSettings,
			Render: func(w io.Writer, p ui.Preview) error {
				view := newSettingsView(previewPage(p, "Borrowed Truths settings"), factorySettings(), settingsErr{}, false)
				return ui.RenderScenario(w, pages, "settings.html", view, p)
			},
		},
	)
	return list
}

func previewPage(p ui.Preview, title string) pageView {
	return pageView{
		Chrome:  ui.Chrome{Title: title, Theme: ui.NormalizeTheme(p.Theme)},
		GameCSS: p.Asset("game.css?v=" + assetVersion),
		GameJS:  p.Asset("game.js?v=" + assetVersion),
	}
}

// previewMatch builds a Game whose engine sits at stage with n seated
// players, and the player each preview phone is drawn for.
func previewMatch(stage string, kind cardKind, n int) (*Game, map[string]games.Player, error) {
	n = max(n, 4)
	s := factorySettings()
	s.MixYours, s.MixBorrowed, s.MixLie = 0, 0, 0
	switch kind {
	case kindBorrowed:
		s.MixBorrowed = 1
	case kindLie:
		s.MixLie = 1
		s.LieSource = lieSourcePlayers
	}
	names := ui.PreviewNames(n)
	rows := make([]rosterRow, n)
	for i := range rows {
		rows[i] = rosterRow{ID: fmt.Sprintf("p%02d", i+1), Name: names[i], Seed: fmt.Sprintf("seed-%d", i+1)}
	}
	e := newEngine(s, rows, bank, rand.New(rand.NewSource(7)), previewNow)
	g := &Game{engine: e, started: true, now: func() time.Time { return previewNow }}
	cast := map[string]games.Player{
		"audience": {ID: "audience", DisplayName: "Audience", Audience: true},
	}
	for i, row := range rows {
		if stage == "facts" && i%3 == 0 {
			continue
		}
		t := previewFacts[i%len(previewFacts)]
		if msg := e.SubmitFacts(row.ID, t[:s.TruthsPerPlayer], t[2:2+s.LiesPerPlayer]); msg != "" {
			return nil, nil, fmt.Errorf("borrowedtruths preview %s: facts %s: %s", stage, row.ID, msg)
		}
	}
	seated := func(id string, host bool) games.Player {
		p := e.player(id)
		return games.Player{ID: p.ID, DisplayName: p.Name, AvatarSeed: p.Seed, Seated: true, ClaimedHost: host}
	}
	if stage == "facts" {
		for _, v := range []string{"teller", "insider", "voter"} {
			cast[v] = seated(rows[0].ID, false)
		}
		cast["host"] = seated(rows[1].ID, true)
		return g, cast, nil
	}
	e.Continue(previewNow)
	teller, insider := e.teller().ID, e.insider()
	voter := ""
	for _, p := range e.voters() {
		if p.ID != insider {
			voter = p.ID
			break
		}
	}
	cast["teller"] = seated(teller, false)
	cast["insider"] = seated(insider, false)
	cast["voter"] = seated(voter, false)
	cast["host"] = seated(voter, true)

	steps := map[string]int{
		"private-read": 0, "public-read": 1, "vote": 3, "vote-closed": 3, "reveal": 4,
		"reveal-lie": 4, "owner-vote": 5, "owner-reveal": 5, "standings": 6, "final": 6,
	}
	for i := 0; i < steps[stage]; i++ {
		e.Continue(previewNow)
		if e.Phase == phaseVote && len(e.Votes) == 0 {
			previewVotes(e, voter, stage == "vote")
		}
	}
	switch stage {
	case "vote-closed":
		if !e.VoteClosed {
			e.Continue(previewNow)
		}
	case "owner-vote", "owner-reveal", "standings", "final":
		previewOwnerVotes(e, voter, stage == "owner-vote")
		if stage != "owner-vote" && e.Phase == phaseOwnerVote {
			e.Continue(previewNow)
		}
		if stage == "standings" || stage == "final" {
			e.Continue(previewNow)
		}
		if stage == "final" {
			for i, p := range e.Players {
				p.Score += (n - i) % 5 * 3
			}
			e.Players[1].Fooled = 3
			e.Players[2].Straight = 1
			e.Players[3].Calls = 4
			e.Tell = len(e.Order) - 1
			e.Continue(previewNow)
		}
	}
	if e.Phase == phaseFinal {
		cast["teller"], cast["insider"] = cast["voter"], cast["voter"]
	}
	return g, cast, nil
}

// previewVotes locks most voters with a spread of picks. open leaves the
// preview voter unlocked so the phone shows the buttons.
func previewVotes(e *engine, voter string, open bool) {
	picks := []string{pickTrue, pickLie, pickBorrowed}
	for i, p := range e.voters() {
		if (open && p.ID == voter) || (open && i%3 == 2) {
			continue
		}
		e.Vote(p.ID, picks[i%3], true)
	}
	e.Crowd["crowd-1"], e.Crowd["crowd-2"], e.Crowd["crowd-3"] = pickTrue, pickLie, pickLie
}

func previewOwnerVotes(e *engine, voter string, open bool) {
	names := e.voters()
	for i, p := range names {
		if open && (p.ID == voter || i%2 == 1) {
			continue
		}
		target := names[(i+1)%len(names)].ID
		if target == p.ID {
			continue
		}
		e.OwnerVote(p.ID, target, previewNow)
	}
}

// previewFacts are two truths and two lies per player.
var previewFacts = [][]string{
	{"I once got locked in a zoo overnight.", "I have a tattoo of a sandwich.", "I was a child chess champion.", "I once met a president at a petrol station."},
	{"I can't whistle and I've tried for thirty years.", "I cried at a toaster advert.", "I own forty rubber ducks.", "I have swum with sharks twice."},
	{"I got my head stuck in a railing on a school trip.", "I have never seen Star Wars.", "I trained as a clown for a month.", "I was in a boy band for one gig."},
	{"I sleepwalked into my neighbour's kitchen.", "I once ate a raw onion for a bet.", "I have a lucky pair of socks from 2009.", "I learned to juggle in a hospital bed."},
	{"I was chased by a goose on my wedding day.", "I name all my plants after wrestlers.", "I was an extra in a horror film and got cut.", "I once won a pie-eating contest by accident."},
}

// previewTIMStages are the moments of a This Is My round. tim-reveal-none is
// a Not theirs photo.
var previewTIMStages = []string{"tim-look", "tim-claims", "tim-questioning", "tim-vote", "tim-reveal", "tim-reveal-none"}

// previewTIMViewers: owner is the claimant who took the photo, or the hidden
// owner on a Not theirs photo. claimant is a claimant who did not.
var previewTIMViewers = []struct {
	name, viewer string
}{
	{"owner", "seated"},
	{"claimant", "seated"},
	{"voter", "seated"},
	{"host", "host"},
	{"audience", "audience"},
}

// previewTIM builds a Game in the first This Is My round with n seated.
func previewTIM(stage string, n int) (*Game, map[string]games.Player, error) {
	n = max(n, 5)
	s := factorySettings()
	s.TIMRounds = "1"
	s.NotTheirsPct = 0
	if stage == "tim-reveal-none" {
		s.NotTheirsPct = 100
	}
	names := ui.PreviewNames(n)
	rows := make([]rosterRow, n)
	for i := range rows {
		rows[i] = rosterRow{ID: fmt.Sprintf("p%02d", i+1), Name: names[i], Seed: fmt.Sprintf("seed-%d", i+1)}
	}
	e := newEngine(s, rows, bank, rand.New(rand.NewSource(11)), previewNow)
	for i, row := range rows {
		f := previewFacts[i%len(previewFacts)]
		e.SubmitFacts(row.ID, f[:2], f[2:3])
		e.SetPhoto(row.ID, fmt.Sprintf("preview-photo-%02d", i+1))
	}
	e.Continue(previewNow)
	for i := 0; i < 200 && e.Phase != phaseLook; i++ {
		e.Continue(previewNow)
	}
	if e.Phase != phaseLook {
		return nil, nil, fmt.Errorf("borrowedtruths preview %s: no This Is My round", stage)
	}
	g := &Game{engine: e, started: true, now: func() time.Time { return previewNow }, photoSrc: previewPhoto}
	owner := e.Photos[e.Round.Photo].Owner
	var claimant, voter string
	for _, id := range e.Round.Claimants {
		if id != owner {
			claimant = id
		}
	}
	for _, p := range e.voters() {
		if p.ID != owner {
			voter = p.ID
			break
		}
	}
	seated := func(id string, host bool) games.Player {
		p := e.player(id)
		return games.Player{ID: p.ID, DisplayName: p.Name, AvatarSeed: p.Seed, Seated: true, ClaimedHost: host}
	}
	cast := map[string]games.Player{
		"owner":    seated(owner, false),
		"claimant": seated(claimant, false),
		"voter":    seated(voter, false),
		"host":     seated(voter, true),
		"audience": {ID: "audience", DisplayName: "Audience", Audience: true},
	}
	steps := map[string]int{"tim-look": 0, "tim-claims": 2, "tim-questioning": 4, "tim-vote": 5, "tim-reveal": 5, "tim-reveal-none": 5}
	for i := 0; i < steps[stage]; i++ {
		e.Continue(previewNow)
	}
	if stage == "tim-vote" || stage == "tim-reveal" || stage == "tim-reveal-none" {
		picks := append(append([]string(nil), e.Round.Claimants...), pickNone)
		for i, p := range e.voters() {
			if stage == "tim-vote" && (p.ID == voter || i%3 == 2) {
				continue
			}
			e.Vote(p.ID, picks[i%len(picks)], true)
		}
		e.Crowd["crowd-1"] = pickNone
		if stage != "tim-vote" {
			if !e.VoteClosed {
				e.Continue(previewNow)
			}
			e.Continue(previewNow)
		}
	}
	return g, cast, nil
}

// previewPhoto is inline art for the photo, since previews have no files.
func previewPhoto(string) template.URL {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 110">` +
		`<defs><linearGradient id="s" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#ff9a5a"/><stop offset="1" stop-color="#ffd23f"/></linearGradient></defs>` +
		`<rect width="160" height="110" fill="url(#s)"/><circle cx="112" cy="40" r="16" fill="#fff4c2"/>` +
		`<path d="M0 80 L40 52 L70 74 L104 46 L160 82 V110 H0 Z" fill="#3d7bff"/>` +
		`<path d="M0 94 Q40 82 80 94 T160 92 V110 H0 Z" fill="#0b1430"/>` +
		`<rect x="30" y="70" width="10" height="16" fill="#0b1430"/><circle cx="35" cy="66" r="9" fill="#2f5a2a"/></svg>`
	return template.URL("data:image/svg+xml;utf8," + url.PathEscape(svg))
}
