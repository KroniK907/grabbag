package borrowedtruths_test

import (
	"testing"

	"github.com/KroniK907/grabbag/internal/games/borrowedtruths"
	"github.com/KroniK907/grabbag/internal/ui/uitest"
)

func TestScenariosRenderAtEverySweepCount(t *testing.T) {
	t.Parallel()
	uitest.RenderAll(t, borrowedtruths.New().Scenarios())
}
