package quips_test

import (
	"testing"

	"github.com/KroniK907/grabbag/internal/games/quips"
	"github.com/KroniK907/grabbag/internal/ui/uitest"
)

func TestScenariosRenderAtEverySweepCount(t *testing.T) {
	t.Parallel()
	uitest.RenderTenant(t, "quips", quips.New())
}
