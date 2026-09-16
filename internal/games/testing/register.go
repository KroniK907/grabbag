package testinggame

import "github.com/KroniK907/hackbox/internal/games"

func init() {
	games.Register(games.Factory{ID: id, New: func() games.Game { return New() }})
}
