package borrowedtruths_test

import (
	"testing"

	"github.com/KroniK907/grabbag/internal/games/borrowedtruths"
	"github.com/KroniK907/grabbag/internal/ui/uitest"
)

func TestScenariosRenderAtEverySweepCount(t *testing.T) {
	t.Parallel()
	uitest.RenderTenant(t, "borrowedtruths", borrowedtruths.New())
}
