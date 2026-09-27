package host

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBenchSeatScopesCookies(t *testing.T) {
	t.Parallel()
	var seen []string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, c := range r.Cookies() {
			seen = append(seen, c.Name+"="+c.Value)
		}
		http.SetCookie(w, &http.Cookie{Name: "grabbag_player", Value: "new"})
		http.SetCookie(w, &http.Cookie{Name: "other", Value: "x"})
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Cookie", "grabbag_player=main; grabbag_player_s2=two; grabbag_player_s3=three; other=x")
	rec := httptest.NewRecorder()
	benchSeatHandler(2, inner).ServeHTTP(rec, req)

	if len(seen) != 2 || seen[0] != "grabbag_player=two" || seen[1] != "other=x" {
		t.Fatalf("inner saw %v", seen)
	}
	got := map[string]string{}
	for _, c := range rec.Result().Cookies() {
		got[c.Name] = c.Value
	}
	if got["grabbag_player_s2"] != "new" || got["other"] != "x" || len(got) != 2 {
		t.Fatalf("set cookies %v", got)
	}
}
