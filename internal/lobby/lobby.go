// Package lobby owns player identity, Join, Leave, and the live roster.
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

// Board writes the current Lobby board with the live roster names.
func (l *Lobby) Board(w http.ResponseWriter, r *http.Request, joinURL string) {
	players, err := l.players(r.Context())
	if err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	l.render(w, "board.html", boardData{
		JoinURL: joinURL,
		Players: players,
	}, http.StatusOK)
}

type boardData struct {
	JoinURL string
	Players []Player
}

func (l *Lobby) boardRoster(w http.ResponseWriter, r *http.Request) {
	players, err := l.players(r.Context())
	if err != nil {
		http.Error(w, "Could not read the roster.", http.StatusInternalServerError)
		return
	}
	l.render(w, "board-roster", boardData{Players: players}, http.StatusOK)
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
	var player Player
	var claimed int
	err := l.sql.QueryRowContext(
		r.Context(),
		`SELECT player_id, display_name, avatar_seed, claimed_host
		 FROM roster
		 WHERE player_id = ?`,
		state.ID,
	).Scan(&player.ID, &player.DisplayName, &player.AvatarSeed, &claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return Player{}, false, nil
	}
	if err != nil {
		return Player{}, false, fmt.Errorf("lobby: find player: %w", err)
	}
	player.ClaimedHost = claimed != 0
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
	player := Player{
		ID:          playerID,
		DisplayName: name,
		AvatarSeed:  avatarSeed,
		ClaimedHost: claimHost,
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO roster
			(player_id, display_name, avatar_seed, claimed_host, pending_designation)
		 VALUES (?, ?, ?, ?, 0)`,
		player.ID,
		player.DisplayName,
		player.AvatarSeed,
		player.ClaimedHost,
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
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("lobby: commit Leave: %w", err)
	}
	return nil
}

func (l *Lobby) players(ctx context.Context) ([]Player, error) {
	rows, err := l.sql.QueryContext(
		ctx,
		`SELECT player_id, display_name, avatar_seed, claimed_host
		 FROM roster
		 ORDER BY rowid`,
	)
	if err != nil {
		return nil, fmt.Errorf("lobby: list roster: %w", err)
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		var player Player
		var claimed int
		if err := rows.Scan(&player.ID, &player.DisplayName, &player.AvatarSeed, &claimed); err != nil {
			return nil, fmt.Errorf("lobby: scan roster: %w", err)
		}
		player.ClaimedHost = claimed != 0
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lobby: read roster: %w", err)
	}
	return players, nil
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
	pending_designation INTEGER NOT NULL DEFAULT 0 CHECK (pending_designation IN (0, 1))
);
CREATE UNIQUE INDEX IF NOT EXISTS one_claimed_host
	ON roster(claimed_host)
	WHERE claimed_host = 1;
CREATE TABLE IF NOT EXISTS host_phone_session (
	player_id TEXT PRIMARY KEY REFERENCES roster(player_id) ON DELETE CASCADE,
	session_id TEXT NOT NULL UNIQUE REFERENCES admin_session(id) ON DELETE CASCADE
);
`)
	if err != nil {
		return fmt.Errorf("lobby: schema: %w", err)
	}
	return nil
}
