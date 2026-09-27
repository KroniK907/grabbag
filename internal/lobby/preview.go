package lobby

import (
	"fmt"
	"html/template"
	"io"

	"github.com/KroniK907/grabbag/internal/ui"
)

const previewJoinURL = "http://192.168.1.20:8654/"

// Scenarios lists Lobby board, join, room, and settings states for the host
// -dev-preview gallery. Views are built from fixed data; no SQL runs.
func Scenarios() []ui.Scenario {
	board := func(name string, min int, build func(p ui.Preview) boardData) ui.Scenario {
		return ui.Scenario{
			Surface: "board", Group: name, Name: name, Viewer: "tv", Frame: ui.FrameTV, MinPlayers: min,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pageTemplates, "board.html", build(p), p)
			},
		}
	}
	join := func(name string, sample bool, edit func(*joinView)) ui.Scenario {
		return ui.Scenario{
			Surface: "join", Group: name, Name: name, Viewer: "guest", Frame: ui.FramePhone, Sample: sample,
			Render: func(w io.Writer, p ui.Preview) error {
				view := joinView{Chrome: ui.Page("GrabBag.gg"), DisplayName: "", AvatarSeed: "preview-seed"}
				edit(&view)
				return ui.RenderScenario(w, pageTemplates, "join.html", view, p)
			},
		}
	}
	room := func(name, viewer string, min int, sample bool, edit func(*roomView, ui.Preview)) ui.Scenario {
		return ui.Scenario{
			Surface: "room", Group: name, Name: name, Viewer: viewer, Frame: ui.FramePhone,
			MinPlayers: min, Sample: sample,
			Render: func(w io.Writer, p ui.Preview) error {
				view := previewRoom(p, viewer)
				edit(&view, p)
				return ui.RenderScenario(w, pageTemplates, "room.html", view, p)
			},
		}
	}
	none := func(*roomView, ui.Preview) {}
	return []ui.Scenario{
		board("empty", 0, func(p ui.Preview) boardData { return previewBoard(0, 0, DefaultSeatCap) }),
		board("room", 1, func(p ui.Preview) boardData { return previewBoard(p.Players, 3, max(p.Players, 8)) }),
		board("full", 1, func(p ui.Preview) boardData { return previewBoard(p.Players, 9, p.Players) }),
		board("game-loaded", 1, func(p ui.Preview) boardData {
			d := previewBoard(p.Players, 2, max(p.Players, 8))
			d.LoadedGame, d.LoadedGameID = "Apples for Humanity", "apples"
			d.HelpPath = "/play/howto"
			d.Buttons = []BoardButton{{Label: "How to play", Path: "/play/howto"}, {Label: "Deck Library", Path: "/play/picker"}}
			return d
		}),
		board("library", 1, func(p ui.Preview) boardData {
			d := previewBoard(p.Players, 0, max(p.Players, 8))
			d.ShowGameLibrary, d.Catalog = true, previewCatalog(p.Players)
			d.LibraryReturn, d.LibraryElementID = "/board", "game-library-board"
			return d
		}),
		board("restore", 1, func(p ui.Preview) boardData {
			d := previewBoard(p.Players, 0, max(p.Players, 8))
			d.RestorePending, d.RestoreNames = true, ui.PreviewNames(p.Players)
			return d
		}),
		board("admin-locked", 0, func(p ui.Preview) boardData {
			d := previewBoard(0, 0, 8)
			d.AdminLocked = true
			return d
		}),
		join("new", true, func(v *joinView) {}),
		join("returning", false, func(v *joinView) { v.DisplayName = "Alexandria-Rose" }),
		join("password", false, func(v *joinView) { v.DisplayName = "Sam"; v.ShowPassword = true }),
		join("error", false, func(v *joinView) { v.DisplayName = "Sam"; v.Error = "That name is already in the room." }),
		join("kicked", false, func(v *joinView) { v.Kicked = true }),
		room("audience", "audience", 1, false, none),
		room("seated", "seated", 1, true, none),
		room("waiting", "waiting", 1, false, none),
		room("ready", "seated", 1, false, func(v *roomView, _ ui.Preview) {
			v.LoadedGameID, v.ShowReady, v.Ready, v.HelpPath = "apples", true, true, "/play/howto"
		}),
		room("host", "host", 1, true, none),
		room("host-drawer", "host", 1, true, func(v *roomView, p ui.Preview) { v.OpenIDs = "host-drawer" }),
		room("host-library", "host", 1, false, func(v *roomView, p ui.Preview) {
			v.ShowGameLibrary, v.Catalog = true, previewCatalog(p.Players)
			v.LibraryReturn, v.LibraryElementID = "/", "game-library-phone"
			v.OpenIDs = "game-library-phone"
		}),
		room("table-full", "host", 1, false, func(v *roomView, p ui.Preview) {
			v.Player.Seated, v.TableFull = false, true
			for _, pl := range v.Players {
				if pl.Seated && pl.ID != v.Player.ID {
					v.BumpCandidates = append(v.BumpCandidates, pl)
				}
			}
		}),
		room("take-host", "seated", 1, false, func(v *roomView, _ ui.Preview) { v.TakeHost = true }),
		room("restore", "host", 1, false, func(v *roomView, _ ui.Preview) {
			v.RestorePending, v.ShowStart, v.ShowReady = true, false, false
		}),
		{
			Surface: "settings", Name: "login", Viewer: "operator", Frame: ui.FramePage,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pageTemplates, "settings-login.html", settingsData{Chrome: ui.Page("GrabBag.gg settings"), LoginError: ""}, p)
			},
		},
		{
			Surface: "settings", Name: "main", Viewer: "operator", Frame: ui.FramePage, MinPlayers: 1, Sample: false,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pageTemplates, "settings.html", previewSettings(p.Players, ""), p)
			},
		},
	}
}

// RenderPlayPhone writes the Lobby play-phone document around a game phone
// body, the way host wraps live game phones. viewer "host" adds the host
// drawer. The preview patches apply to the game body, not this shell.
func RenderPlayPhone(w io.Writer, p ui.Preview, viewer string, body template.HTML) error {
	view := previewRoom(ui.Preview{Theme: p.Theme, Players: 8}, viewer)
	view.Started, view.GameBody = true, body
	view.OpenIDs = p.Open
	return ui.RenderScenario(w, pageTemplates, "play-phone.html", view, ui.Preview{Theme: p.Theme, Open: p.Open})
}

// RenderSettings writes the Lobby /settings page with gameSettings inlined
// under Game Settings, the way host shows a loaded game's settings.
func RenderSettings(w io.Writer, p ui.Preview, gameID string, gameSettings template.HTML) error {
	view := previewSettings(8, gameID)
	view.GameSettings = gameSettings
	return ui.RenderScenario(w, pageTemplates, "settings.html", view, ui.Preview{Theme: p.Theme, Open: p.Open})
}

func previewPlayers(n int) []Player {
	names := ui.PreviewNames(n)
	out := make([]Player, n)
	for i, name := range names {
		out[i] = Player{
			ID: fmt.Sprintf("p%02d", i+1), DisplayName: name, AvatarSeed: fmt.Sprintf("seed-%d", i),
			ClaimedHost: i == 0, Seated: true, Disconnected: i%7 == 6, Ready: i%3 == 0,
		}
	}
	return out
}

func previewWaiting(n, from int) []Player {
	names := ui.PreviewNames(from + n)[from:]
	out := make([]Player, n)
	for i, name := range names {
		out[i] = Player{ID: fmt.Sprintf("w%02d", i+1), DisplayName: name, AvatarSeed: fmt.Sprintf("wait-%d", i), Waiting: true}
	}
	return out
}

func previewBoard(seated, waiting, seatCap int) boardData {
	return boardData{
		Chrome:        ui.Page("GrabBag.gg board"),
		JoinURL:       previewJoinURL,
		Open:          true,
		SeatCap:       seatCap,
		SeatedCount:   seated,
		AudienceCount: waiting + 2,
		Seated:        previewPlayers(seated),
		Waiting:       previewWaiting(waiting, seated),
	}
}

func previewCatalog(seated int) []CatalogEntry {
	return []CatalogEntry{
		{ID: "apples", Name: "Apples for Humanity", Description: "Judge picks the funniest answer card. Every round a new judge.", MinPlayers: 1, PlayerLine: "1+ players", Loaded: true},
		{ID: "quips", Name: "Quick Quips", Description: "Write a quip for two prompts, then the room votes head to head.", MinPlayers: 2, PlayerLine: "2+ players"},
		{ID: "testing", Name: "Testing", Description: "Tap counter and latency readout for checking phones.", MinPlayers: 1, PlayerLine: "1+ players", Warning: fmt.Sprintf("%d seated", seated)},
	}
}

func previewRoom(p ui.Preview, viewer string) roomView {
	n := max(p.Players, 1)
	players := append(previewPlayers(n), previewWaiting(2, n)...)
	me := players[0]
	switch viewer {
	case "audience":
		me = Player{ID: "aud", DisplayName: "Priya", AvatarSeed: "aud-seed"}
		players = append(players, me)
	case "waiting":
		me = players[n]
	case "seated":
		if n > 1 {
			me = players[1]
		} else {
			me.ClaimedHost = false
		}
	}
	view := roomView{
		Chrome: ui.Page("GrabBag.gg room"), Player: me,
		GameIDs: []string{"apples", "quips", "testing"}, LoadedGameID: "",
		Players: players,
	}
	if viewer == "host" {
		view.HostPanel, view.LiveKick, view.ShowStart = true, true, true
	}
	return view
}

func previewSettings(n int, gameID string) settingsData {
	return settingsData{
		Chrome: ui.Page("GrabBag.gg settings"), Open: true, CycleSeats: true, FillEmpty: true,
		AutoPause: true, ProtectHost: true, DisconnectAfter: 30, KickTimeout: 120,
		ResetReadyWhen: "switch", SeatCap: max(n, 8), AdvertisedHostname: "grabbag.local",
		CanMakeHost: true, Players: append(previewPlayers(n), previewWaiting(2, n)...),
		GameIDs: []string{"apples", "quips", "testing"}, LoadedGameID: gameID, LiveKick: true,
	}
}
