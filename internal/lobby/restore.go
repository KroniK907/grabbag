package lobby

import (
	"context"
	"fmt"
	"net/http"
)

// RestorePending reports keep-or-clear after a process start with a roster.
func (l *Lobby) RestorePending() bool {
	l.restoreMu.Lock()
	defer l.restoreMu.Unlock()
	return l.restorePending
}

func (l *Lobby) beginRestoreIfRoster() error {
	if _, err := l.sql.Exec(`UPDATE room_state SET round_active = 0`); err != nil {
		return fmt.Errorf("lobby: clear round on boot: %w", err)
	}
	var n int
	if err := l.sql.QueryRow(`SELECT COUNT(*) FROM roster`).Scan(&n); err != nil {
		return fmt.Errorf("lobby: count roster: %w", err)
	}
	l.restorePending = n > 0
	return nil
}

func (l *Lobby) refusePending(w http.ResponseWriter) bool {
	if !l.RestorePending() {
		return false
	}
	http.Error(w, "Keep or clear the room first.", http.StatusConflict)
	return true
}

func (l *Lobby) keepRoom(w http.ResponseWriter, r *http.Request) {
	if !l.requireAdmin(w, r) {
		return
	}
	l.finishRestore(r.Context(), true)
	l.writeSettingsOKTo(w, r, settingsReturn(r))
}

func (l *Lobby) clearRoom(w http.ResponseWriter, r *http.Request) {
	if !l.requireAdmin(w, r) {
		return
	}
	if err := l.finishRestore(r.Context(), false); err != nil {
		http.Error(w, "Could not clear the room.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, settingsReturn(r), http.StatusSeeOther)
}

func (l *Lobby) finishRestore(ctx context.Context, keep bool) error {
	l.restoreMu.Lock()
	if !l.restorePending {
		l.restoreMu.Unlock()
		return nil
	}
	if !keep {
		if err := l.nightClear(ctx); err != nil {
			l.restoreMu.Unlock()
			return err
		}
	} else {
		l.live.mu.Lock()
		l.live.startedAt = l.live.now()
		l.live.mu.Unlock()
	}
	l.restorePending = false
	after := l.afterRestore
	l.restoreMu.Unlock()
	if after != nil {
		after(ctx, keep)
	}
	l.events.Publish("roster")
	return nil
}

func (l *Lobby) nightClear(ctx context.Context) error {
	tx, err := l.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("lobby: begin night clear: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM admin_session WHERE kind = 'host-phone'`); err != nil {
		return fmt.Errorf("lobby: revoke host-phone sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM roster`); err != nil {
		return fmt.Errorf("lobby: delete roster: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE room_state SET open = 0, host_queue = '', round_active = 0 WHERE id = 1`,
	); err != nil {
		return fmt.Errorf("lobby: close room: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("lobby: commit night clear: %w", err)
	}
	l.resetLivePlayers()
	return nil
}
