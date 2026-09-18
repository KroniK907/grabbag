package host

import (
	"net/http"
	"strings"

	"github.com/KroniK907/grabbag/internal/games"
)

func (rt *runtime) redirectReturn(w http.ResponseWriter, r *http.Request, fallback string) {
	target := strings.TrimSpace(r.PostFormValue("return_to"))
	if target != "/board" && target != "/" {
		target = fallback
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (rt *runtime) loadFailureNotice(w http.ResponseWriter, r *http.Request, message string) {
	rt.notify("board", "error", message, noticeDefaultSeconds)
	rt.operatorNotice(w, r, message)
}

func (rt *runtime) lookupFactory(id string) (games.Factory, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	f, ok := rt.catalog[id]
	return f, ok
}
