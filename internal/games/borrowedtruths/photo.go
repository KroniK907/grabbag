package borrowedtruths

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"image"
	_ "image/gif"  // decode check for uploads
	_ "image/jpeg" // decode check for uploads
	_ "image/png"  // decode check for uploads
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// maxPhotoBytes is the upload cap. Phones downscale to 1600px JPEG first, so
// a real photo lands well under it.
const maxPhotoBytes = 2 << 20

// runPrefix names the per-match photo folder under the game data dir.
const runPrefix = "run-"

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// clearRuns deletes every photo folder under dir. Load uses it to drop
// folders a crash left behind.
func clearRuns(dir string) {
	if dir == "" {
		return
	}
	entries, _ := os.ReadDir(dir)
	for _, ent := range entries {
		if ent.IsDir() && strings.HasPrefix(ent.Name(), runPrefix) {
			_ = os.RemoveAll(filepath.Join(dir, ent.Name()))
		}
	}
}

// dropRunLocked deletes this match's photo folder.
func (g *Game) dropRunLocked() {
	if g.runDir != "" {
		_ = os.RemoveAll(g.runDir)
		g.runDir = ""
	}
}

// postPhoto takes one This Is My photo during facts. It answers 204, or a
// status with a plain-text line the phone shows.
func (g *Game) postPhoto(w http.ResponseWriter, r *http.Request) {
	h := g.helperNow()
	if h == nil {
		http.NotFound(w, r)
		return
	}
	p, ok, err := h.PlayerFromRequest(r)
	if err != nil || !ok {
		http.Error(w, "Sign in to play.", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBytes+64<<10)
	file, _, err := r.FormFile("photo")
	if err != nil {
		http.Error(w, "That photo is over 2 MB.", http.StatusRequestEntityTooLarge)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoBytes+1))
	if err != nil || len(data) > maxPhotoBytes {
		http.Error(w, "That photo is over 2 MB.", http.StatusRequestEntityTooLarge)
		return
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		http.Error(w, "That file is not a photo.", http.StatusUnsupportedMediaType)
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.engine == nil || !g.started {
		http.NotFound(w, r)
		return
	}
	if g.paused {
		http.Error(w, "The match is paused.", http.StatusConflict)
		return
	}
	if g.runDir == "" {
		if h.DataDir() == "" {
			http.Error(w, "Photos are not available.", http.StatusInternalServerError)
			return
		}
		g.runDir = filepath.Join(h.DataDir(), runPrefix+randomID()[:12])
	}
	if err := os.MkdirAll(g.runDir, 0o700); err != nil {
		http.Error(w, "Could not save the photo.", http.StatusInternalServerError)
		return
	}
	id := randomID()
	if err := os.WriteFile(filepath.Join(g.runDir, id), data, 0o600); err != nil {
		http.Error(w, "Could not save the photo.", http.StatusInternalServerError)
		return
	}
	old, msg := g.engine.SetPhoto(p.ID, id)
	if msg != "" {
		_ = os.Remove(filepath.Join(g.runDir, id))
		http.Error(w, msg, http.StatusConflict)
		return
	}
	if old != "" {
		_ = os.Remove(filepath.Join(g.runDir, old))
	}
	g.publishFactsLocked()
	w.WriteHeader(http.StatusNoContent)
}

// getPhoto serves a photo only once it has been on the board.
func (g *Game) getPhoto(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	g.mu.Lock()
	var path string
	if g.engine != nil && g.runDir != "" && g.engine.photoShown(id) {
		path = filepath.Join(g.runDir, id)
	}
	g.mu.Unlock()
	if path == "" {
		http.NotFound(w, r)
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(data)
}
