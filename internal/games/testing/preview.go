package testinggame

import (
	"fmt"
	"io"

	"github.com/KroniK907/grabbag/internal/ui"
)

// Scenarios lists the Testing board, phone, info, and settings states for the
// host -dev-preview gallery.
func (g *Game) Scenarios() []ui.Scenario {
	board := func(name string, paused bool) ui.Scenario {
		return ui.Scenario{
			Surface: "board", Group: name, Name: name, Viewer: "tv", Frame: ui.FrameTV, Shell: ui.ShellBoard, MinPlayers: 1,
			Render: func(w io.Writer, p ui.Preview) error {
				view := boardView{
					Chrome:  ui.Chrome{Title: "Testing"},
					GameCSS: p.Asset("game.css?v=" + cssVersion),
					Paused:  paused,
					ShowEnd: true,
				}
				for i, name := range ui.PreviewNames(p.Players) {
					view.Players = append(view.Players, playerRow{
						ID: fmt.Sprintf("p%02d", i+1), DisplayName: name, AvatarSeed: fmt.Sprintf("seed-%d", i),
						Connected: i%5 != 4, Latency: fmt.Sprintf("%d ms", 12+i*9), Taps: i * 3, Flash: i == 1,
					})
				}
				return ui.RenderScenario(w, pages, "board.html", view, p)
			},
		}
	}
	phone := func(name, viewer string, paused bool) ui.Scenario {
		return ui.Scenario{
			Surface: "phone", Group: name, Name: name + "-" + viewer, Viewer: viewer,
			Frame: ui.FramePhone, Shell: ui.ShellPlayPhone,
			Render: func(w io.Writer, p ui.Preview) error {
				view := phoneView{
					DisplayName: "Alexandria-Rose", Latency: "38 ms", Paused: paused,
					ShowEnd: viewer == "host", GameCSS: p.Asset("game.css?v=" + cssVersion),
				}
				return ui.RenderScenario(w, pages, "phone.html", view, p)
			},
		}
	}
	return []ui.Scenario{
		board("live", false),
		board("paused", true),
		phone("live", "seated", false),
		phone("live", "host", false),
		phone("paused", "seated", true),
		{
			Surface: "phone", Name: "help", Viewer: "seated", Frame: ui.FramePhone, Shell: ui.ShellHelp,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pages, "help", phoneView{}, p)
			},
		},
		{
			Surface: "page", Name: "info", Viewer: "guest", Frame: ui.FramePage,
			Render: func(w io.Writer, p ui.Preview) error {
				view := boardView{Chrome: ui.Chrome{Title: "About Testing"}, GameCSS: p.Asset("game.css?v=" + cssVersion)}
				return ui.RenderScenario(w, pages, "info.html", view, p)
			},
		},
		{
			Surface: "settings", Name: "latency", Viewer: "operator", Frame: ui.FramePage, Shell: ui.ShellSettings,
			Render: func(w io.Writer, p ui.Preview) error {
				return ui.RenderScenario(w, pages, "settings.html", settingsView{}, p)
			},
		},
	}
}
