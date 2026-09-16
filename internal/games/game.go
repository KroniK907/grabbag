// Package games is the compile-time game loader and the host helper contract.
package games

import (
	"net/http"
	"sync"
	"time"
)

// Player is a Lobby row as a game may read it. Games never write these fields.
type Player struct {
	ID               string
	DisplayName      string
	AvatarSeed       string
	Seated           bool
	Waiting          bool
	Audience         bool
	ClaimedHost      bool
	Connected        bool
	LastHeartbeatRTT time.Duration
}

// Helper is the host-owned API a game uses after Load. Games do not parse
// cookies, open host.sqlite, or write the log file.
type Helper interface {
	Seated() []Player
	Waiting() []Player
	Audience() []Player
	Player(id string) (Player, bool)
	PlayerFromRequest(r *http.Request) (Player, bool, error)
	DataDir() string
	KVGet(key string) ([]byte, bool, error)
	KVSet(key string, value []byte) error
	Finish()
	Pause()
	Resume()
	Publish(name string)
	Log(line string)
	// Theme is the room palette id, neon-light or neon-dark. Games stamp it
	// on full documents so the first paint matches /settings.
	Theme() string
	// HasAdmin reports a valid admin session on r. Games use this for board
	// End game. They do not read the admin cookie themselves.
	HasAdmin(r *http.Request) bool
}

// Game is a compiled-in package host can Load, Start, Stop, and Shutdown.
// MinPlayers and MaxPlayers are 0 when the game does not declare a pair.
type Game interface {
	ID() string
	MinPlayers() int
	MaxPlayers() int
	Load(h Helper) error
	Settings() http.Handler
	Start(h Helper) error
	Board(w http.ResponseWriter, r *http.Request)
	Phone(w http.ResponseWriter, r *http.Request)
	Play() http.Handler
	Pause() error
	Resume() error
	Stop() error
	Shutdown() error
}

// Factory constructs one Game value. Catalog is compile-time only.
type Factory struct {
	ID  string
	New func() Game
}

var (
	catalogMu sync.Mutex
	catalog   []Factory
)

// Register adds a compile-time game. Child packages call it from init.
// Host blank-imports those packages so Catalog is populated. games cannot
// import the children itself; they already import this package for Helper.
func Register(f Factory) {
	if f.ID == "" || f.New == nil {
		return
	}
	catalogMu.Lock()
	defer catalogMu.Unlock()
	catalog = append(catalog, f)
}

// Catalog is the compile-time list of registered game packages.
func Catalog() []Factory {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	out := make([]Factory, len(catalog))
	copy(out, catalog)
	return out
}
