package apples_test

import (
	"testing"

	"github.com/KroniK907/grabbag/internal/games/apples"
	"github.com/KroniK907/grabbag/internal/ui/uitest"
)

func TestScenariosRenderAtEverySweepCount(t *testing.T) {
	t.Parallel()
	uitest.RenderAll(t, apples.New().Scenarios())
}
