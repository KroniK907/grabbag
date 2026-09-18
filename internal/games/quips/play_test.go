package quips

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/games"
)

func TestDraftPOSTReturns204(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := newMatchHelper(dir, []games.Player{
		{ID: "p1", DisplayName: "One", Seated: true},
		{ID: "p2", DisplayName: "Two", Seated: true},
	})
	g := New()
	if err := g.Load(h); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Shutdown() })
	if err := g.Start(h); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"slot": {"0"}, "text": {"draft text"}}
	req := httptest.NewRequest(http.MethodPost, "/draft", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "player", Value: "p1"})
	rec := httptest.NewRecorder()
	g.Play().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type matchHelper struct {
	fakeHelper
	playerID string
}

func newMatchHelper(dir string, seated []games.Player) *matchHelper {
	fh := newFakeHelper(dir)
	fh.seated = seated
	return &matchHelper{fakeHelper: *fh, playerID: seated[0].ID}
}

func (h *matchHelper) Seated() []games.Player { return h.seated }
func (h *matchHelper) PlayerFromRequest(r *http.Request) (games.Player, bool, error) {
	for _, p := range h.seated {
		if p.ID == h.playerID {
			return p, true, nil
		}
	}
	return games.Player{}, false, nil
}
