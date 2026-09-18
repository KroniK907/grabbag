package host

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/KroniK907/grabbag/internal/lobby"
)

func formatPlayerLine(min, max int) string {
	var parts []string
	if min > 0 {
		parts = append(parts, fmt.Sprintf("at least %d", min))
	}
	if max > 0 {
		parts = append(parts, fmt.Sprintf("up to %d", max))
	}
	if len(parts) == 0 {
		return ""
	}
	return "Players: " + strings.Join(parts, ", ")
}

func catalogWarning(min, max, seated int) string {
	if min > 0 && seated < min {
		return fmt.Sprintf("Need at least %d seated players.", min)
	}
	if max > 0 && seated > max {
		return fmt.Sprintf("Too many seated players (max %d).", max)
	}
	return ""
}

func loadBlockedMessage(min, max, seated int) string {
	return catalogWarning(min, max, seated)
}

func (rt *runtime) buildCatalog(ctx context.Context, loadedID string) ([]lobby.CatalogEntry, error) {
	seated, err := rt.room.SeatedCount(ctx)
	if err != nil {
		return nil, err
	}
	type row struct {
		name string
		entry lobby.CatalogEntry
	}
	rows := make([]row, 0, len(rt.catalog))
	for id, factory := range rt.catalog {
		game := factory.New()
		name := strings.TrimSpace(game.Name())
		if name == "" {
			name = id
		}
		rows = append(rows, row{
			name: name,
			entry: lobby.CatalogEntry{
				ID:          id,
				Name:        name,
				Description: strings.TrimSpace(game.Description()),
				MinPlayers:  game.MinPlayers(),
				MaxPlayers:  game.MaxPlayers(),
				PlayerLine:  formatPlayerLine(game.MinPlayers(), game.MaxPlayers()),
				Warning:     catalogWarning(game.MinPlayers(), game.MaxPlayers(), seated),
				Loaded:      id == loadedID,
			},
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.EqualFold(rows[i].name, rows[j].name)
	})
	out := make([]lobby.CatalogEntry, len(rows))
	for i := range rows {
		out[i] = rows[i].entry
	}
	return out, nil
}
