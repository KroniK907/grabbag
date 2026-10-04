package testinggame_test

import (
	"testing"

	testinggame "github.com/KroniK907/grabbag/internal/games/testing"
	"github.com/KroniK907/grabbag/internal/ui/uitest"
)

func TestScenariosRenderAtEverySweepCount(t *testing.T) {
	t.Parallel()
	uitest.RenderTenant(t, "testing", testinggame.New())
}
