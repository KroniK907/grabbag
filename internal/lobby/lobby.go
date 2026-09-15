// Package lobby owns player identity, Join, Leave, seats, wait, and the live roster.
package lobby

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/KroniK907/hackbox/internal/platform/hub"
	"github.com/KroniK907/hackbox/internal/store"
)

const (
	// PlayerCookieName is the host-only cookie that identifies a roster player.
	PlayerCookieName = "hackbox_player"

	// SeatCap is the v1 host seat limit. The settings field is later.
	SeatCap = 8

	cookieMaxAge = 30 * 24 * 60 * 60
)

var (
	errNameRequired     = errors.New("lobby: display name is required")
	errNameTaken        = errors.New("lobby: display name is already in use")
	errPasswordMismatch = errors.New("lobby: admin password is incorrect")
)

//go:embed templates/*.html
var templateFiles embed.FS

var pageTemplates = template.Must(template.ParseFS(templateFiles, "templates/*.html"))

// Config supplies the host-owned password and cookie policies used by Lobby.
type Config struct {
	AdminCookieName string
	Events          *hub.Hub
	PasswordMatches func(encodedHash, password string) bool
	SecureCookie    func(*http.Request) bool
}

// Player is a live roster row.
type Player struct {
	ID          string
	DisplayName string
	AvatarSeed  string
	ClaimedHost bool
	Seated      bool
	Waiting     bool
}

// Lobby owns the live roster and its phone writes.
type Lobby struct {
	sql             *sql.DB
	adminCookieName string
	events          *hub.Hub
	passwordMatches func(encodedHash, password string) bool
	secureCookie    func(*http.Request) bool
}

// New creates Lobby and its SQLite tables.
func New(db *store.DB, config Config) (*Lobby, error) {
	if db == nil || db.SQL() == nil {
		return nil, errors.New("lobby: nil store")
	}
	if config.AdminCookieName == "" {
		return nil, errors.New("lobby: empty admin cookie name")
	}
	if config.Events == nil {
		return nil, errors.New("lobby: nil event hub")
	}
	if config.PasswordMatches == nil {
		return nil, errors.New("lobby: nil password matcher")
	}
	if config.SecureCookie == nil {
		return nil, errors.New("lobby: nil secure-cookie policy")
	}
	room := &Lobby{
		sql:             db.SQL(),
		adminCookieName: config.AdminCookieName,
		events:          config.Events,
		passwordMatches: config.PasswordMatches,
		secureCookie:    config.SecureCookie,
	}
	if err := room.ensureSchema(); err != nil {
		return nil, err
	}
	return room, nil
}

// Register adds Lobby-owned stream, partial, and phone write routes to mux.
func (l *Lobby) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /lobby/events", l.events.ServeHTTP)
	mux.HandleFunc("GET /lobby/partials/board-roster", l.boardRoster)
	mux.HandleFunc("GET /lobby/partials/phone", l.phoneBody)
	mux.HandleFunc("POST /lobby/join", l.join)
	mux.HandleFunc("POST /lobby/leave", l.leave)
	mux.HandleFunc("POST /lobby/wait", l.waitToggle)
	mux.HandleFunc("GET /settings", l.settings)
	mux.HandleFunc("POST /settings/open", l.openRoom)
	mux.HandleFunc("POST /settings/close", l.closeRoom)
	mux.HandleFunc("POST /settings/kick", l.kick)
}

// Phone writes the current Lobby phone body. A live player cookie opens the
// in-room body. Other requests get Join, with stale cookie details prefilled.
func (l *Lobby) Phone(w http.ResponseWriter, r *http.Request) {
	player, ok, err := l.PlayerFromRequest(r)
	if err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	if ok {
		l.render(w, "room.html", player, http.StatusOK)
		return
	}
	l.writeJoin(w, r, "join.html", "", "", http.StatusOK)
}

// Board writes the current Lobby board with seated names, then the wait list.
func (l *Lobby) Board(w http.ResponseWriter, r *http.Request, joinURL string) {
	data, err := l.boardData(r.Context())
	if err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	data.JoinURL = joinURL
	l.render(w, "board.html", data, http.StatusOK)
}

type boardData struct {
	JoinURL string
	Seated  []Player
	Waiting []Player
}

type settingsData struct {
	Open    bool
	Players []Player
}

func (l *Lobby) boardRoster(w http.ResponseWriter, r *http.Request) {
	data, err := l.boardData(r.Context())
	if err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	l.render(w, "board-roster", data, http.StatusOK)
}

func (l *Lobby) boardData(ctx context.Context) (boardData, error) {
	seated, err := l.listPlayers(ctx, `seated = 1 ORDER BY rowid`)
	if err != nil {
		return boardData{}, err
	}
	waiting, err := l.listPlayers(ctx, `waiting = 1 ORDER BY wait_seq`)
	if err != nil {
		return boardData{}, err
	}
	return boardData{Seated: seated, Waiting: waiting}, nil
}

func (l *Lobby) phoneBody(w http.ResponseWriter, r *http.Request) {
	player, ok, err := l.PlayerFromRequest(r)
	if err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	if ok {
		l.render(w, "room-body", player, http.StatusOK)
		return
	}
	l.writeJoin(w, r, "join-body", "", "", http.StatusOK)
}

// PlayerFromRequest resolves player identity only from the player cookie.
func (l *Lobby) PlayerFromRequest(r *http.Request) (Player, bool, error) {
	state, ok := playerCookieFromRequest(r)
	if !ok || state.ID == "" {
		return Player{}, false, nil
	}
	player, err := scanPlayer(l.sql.QueryRowContext(
		r.Context(),
		playerSelect+` WHERE player_id = ?`,
		state.ID,
	).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Player{}, false, nil
	}
	if err != nil {
		return Player{}, false, fmt.Errorf("lobby: find player: %w", err)
	}
	return player, true, nil
}

func (l *Lobby) join(w http.ResponseWriter, r *http.Request) {
	if _, ok, err := l.PlayerFromRequest(r); err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	} else if ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		l.writeJoin(w, r, "join.html", "", "Could not read the form.", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.PostFormValue("display_name"))
	if name == "" {
		l.writeJoin(w, r, "join.html", name, "Name is required.", http.StatusBadRequest)
		return
	}

	staleCookie, _ := playerCookieFromRequest(r)
	result, err := l.addPlayer(
		r.Context(),
		name,
		staleCookie.AvatarSeed,
		r.PostFormValue("admin_password"),
	)
	if err != nil {
		switch {
		case errors.Is(err, errNameRequired):
			l.writeJoin(w, r, "join.html", name, "Name is required.", http.StatusBadRequest)
		case errors.Is(err, errNameTaken):
			l.writeJoin(w, r, "join.html", name, "That name is already in use.", http.StatusConflict)
		case errors.Is(err, errPasswordMismatch):
			l.writeJoin(w, r, "join.html", name, "Admin password is incorrect.", http.StatusUnauthorized)
		default:
			http.Error(w, "Could not join the room.", http.StatusInternalServerError)
		}
		return
	}

	state := playerCookie{
		ID:          result.player.ID,
		DisplayName: result.player.DisplayName,
		AvatarSeed:  result.player.AvatarSeed,
	}
	value, err := encodePlayerCookie(state)
	if err != nil {
		http.Error(w, "Could not join the room.", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, l.cookie(r, PlayerCookieName, value))
	if result.adminSessionID != "" {
		http.SetCookie(w, l.cookie(r, l.adminCookieName, result.adminSessionID))
	}
	l.events.Publish("roster")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (l *Lobby) leave(w http.ResponseWriter, r *http.Request) {
	player, ok, err := l.PlayerFromRequest(r)
	if err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := l.removePlayer(r.Context(), player.ID); err != nil {
		http.Error(w, "Could not leave the room.", http.StatusInternalServerError)
		return
	}
	l.events.Publish("roster")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type joinResult struct {
	player         Player
	adminSessionID string
}

func (l *Lobby) addPlayer(ctx context.Context, name, avatarSeed, password string) (joinResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return joinResult{}, errNameRequired
	}
	tx, err := l.sql.BeginTx(ctx, nil)
	if err != nil {
		return joinResult{}, fmt.Errorf("lobby: begin Join: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var nameExists int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM roster WHERE display_name = ? COLLATE NOCASE)`,
		name,
	).Scan(&nameExists); err != nil {
		return joinResult{}, fmt.Errorf("lobby: check display name: %w", err)
	}
	if nameExists != 0 {
		return joinResult{}, errNameTaken
	}

	var hostExists int
	if err := tx.QueryRowContext(
		ctx,
		`SELECT EXISTS(SELECT 1 FROM roster WHERE claimed_host = 1)`,
	).Scan(&hostExists); err != nil {
		return joinResult{}, fmt.Errorf("lobby: check claimed host: %w", err)
	}

	claimHost := hostExists == 0 && password != ""
	if claimHost {
		var hash string
		if err := tx.QueryRowContext(ctx, `SELECT hash FROM admin_password WHERE id = 1`).Scan(&hash); err != nil {
			return joinResult{}, fmt.Errorf("lobby: read admin hash: %w", err)
		}
		if !l.passwordMatches(hash, password) {
			return joinResult{}, errPasswordMismatch
		}
	}

	playerID, err := newUUID()
	if err != nil {
		return joinResult{}, err
	}
	if avatarSeed == "" {
		avatarSeed, err = newRandomValue(16)
		if err != nil {
			return joinResult{}, fmt.Errorf("lobby: make avatar seed: %w", err)
		}
	}
	if err := fillWait(ctx, tx); err != nil {
		return joinResult{}, err
	}
	seated, waiting, waitSeq, err := seatOnJoin(ctx, tx, claimHost)
	if err != nil {
		return joinResult{}, err
	}
	player := Player{
		ID:          playerID,
		DisplayName: name,
		AvatarSeed:  avatarSeed,
		ClaimedHost: claimHost,
		Seated:      seated,
		Waiting:     waiting,
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO roster
			(player_id, display_name, avatar_seed, claimed_host, pending_designation, seated, waiting, wait_seq)
		 VALUES (?, ?, ?, ?, 0, ?, ?, ?)`,
		player.ID,
		player.DisplayName,
		player.AvatarSeed,
		player.ClaimedHost,
		boolToInt(seated),
		boolToInt(waiting),
		waitSeq,
	); err != nil {
		return joinResult{}, fmt.Errorf("lobby: insert roster player: %w", err)
	}

	result := joinResult{player: player}
	if claimHost {
		result.adminSessionID, err = newRandomValue(32)
		if err != nil {
			return joinResult{}, fmt.Errorf("lobby: make host-phone session: %w", err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO admin_session (id, kind) VALUES (?, 'host-phone')`,
			result.adminSessionID,
		); err != nil {
			return joinResult{}, fmt.Errorf("lobby: insert host-phone session: %w", err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO host_phone_session (player_id, session_id) VALUES (?, ?)`,
			player.ID,
			result.adminSessionID,
		); err != nil {
			return joinResult{}, fmt.Errorf("lobby: link host-phone session: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return joinResult{}, fmt.Errorf("lobby: commit Join: %w", err)
	}
	return result, nil
}

func (l *Lobby) removePlayer(ctx context.Context, playerID string) error {
	tx, err := l.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("lobby: begin Leave: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM admin_session
		 WHERE id IN (
			SELECT session_id FROM host_phone_session WHERE player_id = ?
		 )`,
		playerID,
	); err != nil {
		return fmt.Errorf("lobby: revoke host-phone session: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM roster WHERE player_id = ?`, playerID); err != nil {
		return fmt.Errorf("lobby: delete roster player: %w", err)
	}
	if err := fillWait(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("lobby: commit Leave: %w", err)
	}
	return nil
}

func (l *Lobby) settings(w http.ResponseWriter, r *http.Request) {
	open, err := l.roomOpen(r.Context())
	if err != nil {
		http.Error(w, "Could not read the room.", http.StatusInternalServerError)
		return
	}
	players, err := l.listPlayers(r.Context(), `1 = 1 ORDER BY rowid`)
	if err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	l.render(w, "settings.html", settingsData{Open: open, Players: players}, http.StatusOK)
}

func (l *Lobby) openRoom(w http.ResponseWriter, r *http.Request) {
	if !l.requireAdmin(w, r) {
		return
	}
	if err := l.setRoomOpen(r.Context(), true); err != nil {
		http.Error(w, "Could not open the room.", http.StatusInternalServerError)
		return
	}
	l.events.Publish("roster")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (l *Lobby) closeRoom(w http.ResponseWriter, r *http.Request) {
	if !l.requireAdmin(w, r) {
		return
	}
	if err := l.setRoomOpen(r.Context(), false); err != nil {
		http.Error(w, "Could not close the room.", http.StatusInternalServerError)
		return
	}
	l.events.Publish("roster")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (l *Lobby) kick(w http.ResponseWriter, r *http.Request) {
	if !l.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	playerID := r.PostFormValue("player_id")
	if playerID == "" {
		http.Error(w, "Player is required.", http.StatusBadRequest)
		return
	}
	if err := l.removePlayer(r.Context(), playerID); err != nil {
		http.Error(w, "Could not kick the player.", http.StatusInternalServerError)
		return
	}
	l.events.Publish("roster")
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (l *Lobby) waitToggle(w http.ResponseWriter, r *http.Request) {
	player, ok, err := l.PlayerFromRequest(r)
	if err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if player.Seated {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := l.toggleWait(r.Context(), player); err != nil {
		http.Error(w, "Could not update the wait list.", http.StatusInternalServerError)
		return
	}
	l.events.Publish("roster")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (l *Lobby) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(l.adminCookieName)
	if err != nil || cookie.Value == "" {
		http.Error(w, "Admin session required.", http.StatusUnauthorized)
		return false
	}
	var n int
	if err := l.sql.QueryRowContext(
		r.Context(),
		`SELECT COUNT(*) FROM admin_session WHERE id = ?`,
		cookie.Value,
	).Scan(&n); err != nil {
		http.Error(w, "Could not read the session.", http.StatusInternalServerError)
		return false
	}
	if n == 0 {
		http.Error(w, "Admin session required.", http.StatusUnauthorized)
		return false
	}
	return true
}

func (l *Lobby) setRoomOpen(ctx context.Context, open bool) error {
	tx, err := l.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("lobby: begin room open: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE room_state SET open = ? WHERE id = 1`, boolToInt(open)); err != nil {
		return fmt.Errorf("lobby: update room open: %w", err)
	}
	if open {
		if err := fillWait(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("lobby: commit room open: %w", err)
	}
	return nil
}

func (l *Lobby) toggleWait(ctx context.Context, player Player) error {
	if player.Seated {
		return nil
	}
	tx, err := l.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("lobby: begin wait-toggle: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if player.Waiting {
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE roster SET waiting = 0, wait_seq = NULL WHERE player_id = ?`,
			player.ID,
		); err != nil {
			return fmt.Errorf("lobby: drop wait: %w", err)
		}
	} else {
		seq, err := nextWaitSeq(ctx, tx)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE roster SET waiting = 1, wait_seq = ? WHERE player_id = ? AND seated = 0`,
			seq,
			player.ID,
		); err != nil {
			return fmt.Errorf("lobby: join wait: %w", err)
		}
		if err := fillWait(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("lobby: commit wait-toggle: %w", err)
	}
	return nil
}

func (l *Lobby) roomOpen(ctx context.Context) (bool, error) {
	var open int
	if err := l.sql.QueryRowContext(ctx, `SELECT open FROM room_state WHERE id = 1`).Scan(&open); err != nil {
		return false, fmt.Errorf("lobby: read room open: %w", err)
	}
	return open != 0, nil
}

func (l *Lobby) listPlayers(ctx context.Context, where string) ([]Player, error) {
	rows, err := l.sql.QueryContext(ctx, playerSelect+` WHERE `+where)
	if err != nil {
		return nil, fmt.Errorf("lobby: list roster: %w", err)
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		player, err := scanPlayer(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("lobby: scan roster: %w", err)
		}
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lobby: read roster: %w", err)
	}
	return players, nil
}

const playerSelect = `SELECT player_id, display_name, avatar_seed, claimed_host, seated, waiting FROM roster`

func scanPlayer(scan func(dest ...any) error) (Player, error) {
	var player Player
	var claimed, seated, waiting int
	if err := scan(&player.ID, &player.DisplayName, &player.AvatarSeed, &claimed, &seated, &waiting); err != nil {
		return Player{}, err
	}
	player.ClaimedHost = claimed != 0
	player.Seated = seated != 0
	player.Waiting = waiting != 0
	return player, nil
}

func seatOnJoin(ctx context.Context, tx *sql.Tx, claimHost bool) (seated bool, waiting bool, waitSeq any, err error) {
	if claimHost {
		return true, false, nil, nil
	}
	open, err := roomOpenTx(ctx, tx)
	if err != nil {
		return false, false, nil, err
	}
	if open {
		count, err := seatedCount(ctx, tx)
		if err != nil {
			return false, false, nil, err
		}
		if count < SeatCap {
			return true, false, nil, nil
		}
	}
	seq, err := nextWaitSeq(ctx, tx)
	if err != nil {
		return false, false, nil, err
	}
	return false, true, seq, nil
}

func fillWait(ctx context.Context, tx *sql.Tx) error {
	open, err := roomOpenTx(ctx, tx)
	if err != nil {
		return err
	}
	if !open {
		return nil
	}
	for {
		count, err := seatedCount(ctx, tx)
		if err != nil {
			return err
		}
		if count >= SeatCap {
			return nil
		}
		var playerID string
		err = tx.QueryRowContext(
			ctx,
			`SELECT player_id FROM roster WHERE waiting = 1 ORDER BY wait_seq LIMIT 1`,
		).Scan(&playerID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("lobby: next waiter: %w", err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE roster SET seated = 1, waiting = 0, wait_seq = NULL WHERE player_id = ?`,
			playerID,
		); err != nil {
			return fmt.Errorf("lobby: seat waiter: %w", err)
		}
	}
}

func roomOpenTx(ctx context.Context, tx *sql.Tx) (bool, error) {
	var open int
	if err := tx.QueryRowContext(ctx, `SELECT open FROM room_state WHERE id = 1`).Scan(&open); err != nil {
		return false, fmt.Errorf("lobby: read room open: %w", err)
	}
	return open != 0, nil
}

func seatedCount(ctx context.Context, tx *sql.Tx) (int, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM roster WHERE seated = 1`).Scan(&count); err != nil {
		return 0, fmt.Errorf("lobby: count seated: %w", err)
	}
	return count, nil
}

func nextWaitSeq(ctx context.Context, tx *sql.Tx) (int64, error) {
	var seq sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(wait_seq) FROM roster`).Scan(&seq); err != nil {
		return 0, fmt.Errorf("lobby: next wait seq: %w", err)
	}
	if !seq.Valid {
		return 1, nil
	}
	return seq.Int64 + 1, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (l *Lobby) writeJoin(
	w http.ResponseWriter,
	r *http.Request,
	templateName string,
	submittedName string,
	message string,
	status int,
) {
	state, _ := playerCookieFromRequest(r)
	if submittedName != "" {
		state.DisplayName = submittedName
	}
	var hostExists int
	if err := l.sql.QueryRowContext(
		r.Context(),
		`SELECT EXISTS(SELECT 1 FROM roster WHERE claimed_host = 1)`,
	).Scan(&hostExists); err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	l.render(w, templateName, struct {
		DisplayName  string
		ShowPassword bool
		Error        string
	}{
		DisplayName:  state.DisplayName,
		ShowPassword: hostExists == 0,
		Error:        message,
	}, status)
}

func (l *Lobby) cookie(r *http.Request, name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   cookieMaxAge,
		HttpOnly: true,
		Secure:   l.secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	}
}

func (l *Lobby) render(w http.ResponseWriter, name string, data any, status int) {
	var body bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&body, name, data); err != nil {
		http.Error(w, "Could not render the page.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = body.WriteTo(w)
}

type playerCookie struct {
	ID          string `json:"id"`
	DisplayName string `json:"name"`
	AvatarSeed  string `json:"avatar"`
}

func playerCookieFromRequest(r *http.Request) (playerCookie, bool) {
	cookie, err := r.Cookie(PlayerCookieName)
	if err != nil {
		return playerCookie{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return playerCookie{}, false
	}
	var state playerCookie
	if err := json.Unmarshal(raw, &state); err != nil {
		return playerCookie{}, false
	}
	return state, true
}

func encodePlayerCookie(state playerCookie) (string, error) {
	raw, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("lobby: encode player cookie: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("lobby: make player UUID: %w", err)
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	encoded := hex.EncodeToString(value[:])
	return encoded[0:8] + "-" +
		encoded[8:12] + "-" +
		encoded[12:16] + "-" +
		encoded[16:20] + "-" +
		encoded[20:32], nil
}

func newRandomValue(length int) (string, error) {
	value := make([]byte, length)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (l *Lobby) ensureSchema() error {
	_, err := l.sql.Exec(`
CREATE TABLE IF NOT EXISTS roster (
	player_id TEXT PRIMARY KEY CHECK (length(player_id) > 0),
	display_name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK (length(trim(display_name)) > 0),
	avatar_seed TEXT NOT NULL CHECK (length(avatar_seed) > 0),
	claimed_host INTEGER NOT NULL DEFAULT 0 CHECK (claimed_host IN (0, 1)),
	pending_designation INTEGER NOT NULL DEFAULT 0 CHECK (pending_designation IN (0, 1)),
	seated INTEGER NOT NULL DEFAULT 0 CHECK (seated IN (0, 1)),
	waiting INTEGER NOT NULL DEFAULT 0 CHECK (waiting IN (0, 1)),
	wait_seq INTEGER,
	CHECK (NOT (seated = 1 AND waiting = 1))
);
CREATE UNIQUE INDEX IF NOT EXISTS one_claimed_host
	ON roster(claimed_host)
	WHERE claimed_host = 1;
CREATE TABLE IF NOT EXISTS host_phone_session (
	player_id TEXT PRIMARY KEY REFERENCES roster(player_id) ON DELETE CASCADE,
	session_id TEXT NOT NULL UNIQUE REFERENCES admin_session(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS room_state (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	open INTEGER NOT NULL DEFAULT 0 CHECK (open IN (0, 1))
);
INSERT OR IGNORE INTO room_state (id, open) VALUES (1, 0);
`)
	if err != nil {
		return fmt.Errorf("lobby: schema: %w", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE roster ADD COLUMN seated INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE roster ADD COLUMN waiting INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE roster ADD COLUMN wait_seq INTEGER`,
	} {
		if _, err := l.sql.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("lobby: schema: %w", err)
		}
	}
	if _, err := l.sql.Exec(`
UPDATE roster
SET seated = 1, waiting = 0, wait_seq = NULL
WHERE claimed_host = 1 AND seated = 0 AND waiting = 0;
`); err != nil {
		return fmt.Errorf("lobby: schema: %w", err)
	}
	if _, err := l.sql.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS wait_order
	ON roster(wait_seq)
	WHERE waiting = 1;
`); err != nil {
		return fmt.Errorf("lobby: schema: %w", err)
	}
	return nil
}
