package host

import (
	"bytes"
	"net/http"
	"strings"
)

func (rt *runtime) registerGameRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /settings/load", rt.postLoad)
	mux.HandleFunc("POST /settings/start", rt.postStart)
	mux.HandleFunc("POST /settings/stop", rt.postStop)
	mux.HandleFunc("POST /settings/shutdown", rt.postShutdown)
	mux.HandleFunc("POST /settings/pause", rt.postPause)
	mux.HandleFunc("POST /settings/resume", rt.postResume)
	mux.HandleFunc("POST /settings/clear-game-data", rt.postClearKV)
	mux.Handle("/settings/game/", http.HandlerFunc(rt.gameSettings))
	mux.Handle("/play/", http.HandlerFunc(rt.play))
}

func (rt *runtime) phone(w http.ResponseWriter, r *http.Request) {
	rt.mu.Lock()
	started, game := rt.started, rt.game
	rt.mu.Unlock()
	if !started || game == nil {
		rt.room.Phone(w, r)
		return
	}
	_, ok, err := rt.room.PlayerFromRequest(r)
	if err != nil || !ok {
		rt.room.Phone(w, r)
		return
	}
	var buf bytes.Buffer
	rec := &capture{buf: &buf, header: make(http.Header)}
	game.Phone(rec, r)
	rt.wrapPhone(w, r, buf.Bytes())
}

func (rt *runtime) refusePending(w http.ResponseWriter) bool {
	if rt.room == nil || !rt.room.RestorePending() {
		return false
	}
	http.Error(w, "Keep or clear the room first.", http.StatusConflict)
	return true
}

func (rt *runtime) board(w http.ResponseWriter, r *http.Request, joinURL string) {
	if rt.room.RestorePending() {
		rt.room.Board(w, r, joinURL)
		return
	}
	rt.mu.Lock()
	started, game := rt.started, rt.game
	rt.mu.Unlock()
	if !started || game == nil {
		rt.room.Board(w, r, joinURL)
		return
	}
	game.Board(w, r)
}

func (rt *runtime) postLoad(w http.ResponseWriter, r *http.Request) {
	if !rt.requireAdmin(w, r) {
		return
	}
	if rt.refusePending(w) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	if err := rt.load(r.Context(), strings.TrimSpace(r.PostFormValue("game_id"))); err != nil {
		rt.operatorNotice(w, r, "Could not load that game.")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (rt *runtime) postStart(w http.ResponseWriter, r *http.Request) {
	if !rt.requireAdmin(w, r) {
		return
	}
	if rt.refusePending(w) {
		return
	}
	if err := rt.start(r.Context()); err != nil {
		msg := "Could not start."
		if err == errStartRefused || err == errNotLoaded {
			msg = "Start needs a loaded game and at least one seated player."
		}
		rt.operatorNotice(w, r, msg)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (rt *runtime) postStop(w http.ResponseWriter, r *http.Request) {
	if !rt.requireAdmin(w, r) {
		return
	}
	if rt.refusePending(w) {
		return
	}
	if err := rt.stop(r.Context(), r.PostFormValue("graceful") == "1"); err != nil {
		http.Error(w, "Could not stop.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (rt *runtime) postShutdown(w http.ResponseWriter, r *http.Request) {
	if !rt.requireAdmin(w, r) {
		return
	}
	if rt.refusePending(w) {
		return
	}
	if err := rt.shutdown(r.Context()); err != nil {
		http.Error(w, "Could not unload the game.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (rt *runtime) postPause(w http.ResponseWriter, r *http.Request) {
	if !rt.requireAdmin(w, r) {
		return
	}
	if rt.refusePending(w) {
		return
	}
	if err := rt.pause(); err != nil {
		http.Error(w, "Could not pause.", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (rt *runtime) postResume(w http.ResponseWriter, r *http.Request) {
	if !rt.requireAdmin(w, r) {
		return
	}
	if rt.refusePending(w) {
		return
	}
	if err := rt.resume(); err != nil {
		http.Error(w, "Could not resume.", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (rt *runtime) postClearKV(w http.ResponseWriter, r *http.Request) {
	if !rt.requireAdmin(w, r) {
		return
	}
	rt.mu.Lock()
	id := rt.loadedID
	rt.mu.Unlock()
	if id == "" {
		http.Error(w, "No game loaded.", http.StatusConflict)
		return
	}
	if err := rt.db.KVDeleteGame(r.Context(), id); err != nil {
		http.Error(w, "Could not clear game data.", http.StatusInternalServerError)
		return
	}
	rt.log.Write("Clear game data " + id)
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (rt *runtime) gameSettings(w http.ResponseWriter, r *http.Request) {
	if !rt.requireAdmin(w, r) {
		return
	}
	rt.mu.Lock()
	game := rt.game
	rt.mu.Unlock()
	if game == nil || game.Settings() == nil {
		http.NotFound(w, r)
		return
	}
	http.StripPrefix("/settings/game", game.Settings()).ServeHTTP(w, r)
}

func (rt *runtime) play(w http.ResponseWriter, r *http.Request) {
	rt.mu.Lock()
	started, game := rt.started, rt.game
	rt.mu.Unlock()
	if game == nil || game.Play() == nil {
		http.NotFound(w, r)
		return
	}
	if !started && r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	http.StripPrefix("/play", game.Play()).ServeHTTP(w, r)
}

func (rt *runtime) hasAdmin(r *http.Request) bool {
	cookie, err := r.Cookie(adminCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	ok, err := rt.db.HasAdminSession(r.Context(), cookie.Value)
	return err == nil && ok
}

func (rt *runtime) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !rt.hasAdmin(r) {
		http.Error(w, "Admin session required.", http.StatusUnauthorized)
		return false
	}
	return true
}
