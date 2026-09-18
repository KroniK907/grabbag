package quips

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/KroniK907/grabbag/internal/games"
)

const (
	stateDirName    = "state"
	burnedFileName  = "burned.json"
	discardFileName = "discard.json"
	kindPrompt      = "prompt"
)

type burnEntry struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type burnedDoc struct {
	FormatVersion int         `json:"formatVersion"`
	Burns         []burnEntry `json:"burns"`
}

type playedRow struct {
	LibraryID string `json:"libraryId"`
	CardID    string `json:"cardId"`
}

type discardDoc struct {
	FormatVersion int         `json:"formatVersion"`
	Played        []playedRow `json:"played"`
}

type burnFace struct {
	Kind    string
	Text    string
	Norm    string
	Burned  bool
	Checked bool
}

type fileWriter struct {
	path    string
	log     func(string)
	mu      sync.Mutex
	cond    *sync.Cond
	pending []byte
	has     bool
	gen     uint64
	doneGen uint64
	closed  bool
	alive   bool
}

func newFileWriter(path string, log func(string)) *fileWriter {
	w := &fileWriter{path: path, log: log, alive: true}
	w.cond = sync.NewCond(&w.mu)
	go w.loop()
	return w
}

func (w *fileWriter) loop() {
	for {
		w.mu.Lock()
		for !w.has && !w.closed {
			w.cond.Wait()
		}
		if w.closed && !w.has {
			w.alive = false
			w.cond.Broadcast()
			w.mu.Unlock()
			return
		}
		raw := w.pending
		gen := w.gen
		w.has = false
		w.pending = nil
		w.mu.Unlock()
		if err := atomicWrite(w.path, raw); err != nil && w.log != nil {
			w.log(fmt.Sprintf("could not write %s: %v", filepath.Base(w.path), err))
		}
		w.mu.Lock()
		w.doneGen = gen
		w.cond.Broadcast()
		w.mu.Unlock()
	}
}

func (w *fileWriter) enqueue(raw []byte) uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return w.doneGen
	}
	w.pending = append([]byte(nil), raw...)
	w.has = true
	w.gen++
	w.cond.Signal()
	return w.gen
}

func (w *fileWriter) wait(gen uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for w.doneGen < gen && !w.closed {
		w.cond.Wait()
	}
}

func (w *fileWriter) close() {
	w.mu.Lock()
	w.closed = true
	w.cond.Broadcast()
	for w.alive {
		w.cond.Wait()
	}
	w.mu.Unlock()
}

func atomicWrite(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func playedKey(libraryID, cardID string) string {
	return libraryID + "\x00" + cardID
}

func (g *Game) openJournalsLocked(h games.Helper) {
	g.stopWritersLocked()
	g.helper = h
	g.burns = nil
	g.burnCorrupt = false
	g.played = map[string]playedRow{}
	g.discardCorrupt = false
	dir := filepath.Join(h.DataDir(), stateDirName)
	_ = os.MkdirAll(dir, 0o700)
	log := h.Log
	g.burnWriter = newFileWriter(filepath.Join(dir, burnedFileName), log)
	g.discardWriter = newFileWriter(filepath.Join(dir, discardFileName), log)
	g.readBurnedLocked(filepath.Join(dir, burnedFileName))
	g.readDiscardLocked(filepath.Join(dir, discardFileName))
}

func (g *Game) stopWritersLocked() {
	if g.burnWriter != nil {
		g.enqueueBurnsLocked()
		g.burnWriter.close()
		g.burnWriter = nil
	}
	if g.discardWriter != nil {
		g.enqueueDiscardLocked()
		g.discardWriter.close()
		g.discardWriter = nil
	}
}

func (g *Game) drainJournalsLocked() {
	if g.burnWriter != nil {
		gen := g.enqueueBurnsLocked()
		g.burnWriter.wait(gen)
	}
	if g.discardWriter != nil {
		gen := g.enqueueDiscardLocked()
		g.discardWriter.wait(gen)
	}
}

func (g *Game) readBurnedLocked(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			g.burns = nil
			return
		}
		g.burnCorrupt = true
		g.logLine("burned.json is unreadable")
		return
	}
	var doc burnedDoc
	if err := json.Unmarshal(raw, &doc); err != nil || doc.FormatVersion != 1 {
		g.burnCorrupt = true
		g.logLine("burned.json is unreadable")
		return
	}
	g.burns = collapseBurns(doc.Burns)
}

func (g *Game) readDiscardLocked(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			g.played = map[string]playedRow{}
			return
		}
		g.discardCorrupt = true
		g.logLine("discard.json is unreadable")
		return
	}
	var doc discardDoc
	if err := json.Unmarshal(raw, &doc); err != nil || doc.FormatVersion != 1 {
		g.discardCorrupt = true
		g.logLine("discard.json is unreadable")
		return
	}
	g.played = collapsePlayed(doc.Played)
}

func (g *Game) logLine(line string) {
	if g.helper != nil {
		g.helper.Log(line)
	}
}

func collapseBurns(in []burnEntry) []burnEntry {
	last := map[string]burnEntry{}
	var order []string
	for _, e := range in {
		kind := e.Kind
		text := normalizeCardText(e.Text)
		if kind != kindPrompt || text == "" {
			continue
		}
		key := kind + "\x00" + text
		if _, ok := last[key]; ok {
			filtered := order[:0]
			for _, k := range order {
				if k != key {
					filtered = append(filtered, k)
				}
			}
			order = filtered
		}
		order = append(order, key)
		last[key] = burnEntry{Kind: kind, Text: text}
	}
	out := make([]burnEntry, 0, len(order))
	for _, k := range order {
		out = append(out, last[k])
	}
	return out
}

func collapsePlayed(in []playedRow) map[string]playedRow {
	out := map[string]playedRow{}
	for _, r := range in {
		if r.LibraryID == "" || r.CardID == "" {
			continue
		}
		out[playedKey(r.LibraryID, r.CardID)] = r
	}
	return out
}

func (g *Game) enqueueBurnsLocked() uint64 {
	if g.burnWriter == nil || g.burnCorrupt {
		return 0
	}
	raw, err := json.MarshalIndent(burnedDoc{FormatVersion: 1, Burns: g.burns}, "", "  ")
	if err != nil {
		return 0
	}
	return g.burnWriter.enqueue(raw)
}

func (g *Game) enqueueDiscardLocked() uint64 {
	if g.discardWriter == nil || g.discardCorrupt {
		return 0
	}
	rows := make([]playedRow, 0, len(g.played))
	for _, r := range g.played {
		rows = append(rows, r)
	}
	raw, err := json.MarshalIndent(discardDoc{FormatVersion: 1, Played: rows}, "", "  ")
	if err != nil {
		return 0
	}
	return g.discardWriter.enqueue(raw)
}

func (g *Game) recordPlayedLocked(libraryID, cardID string) {
	if libraryID == "" || cardID == "" {
		return
	}
	if g.played == nil {
		g.played = map[string]playedRow{}
	}
	g.played[playedKey(libraryID, cardID)] = playedRow{LibraryID: libraryID, CardID: cardID}
}

func (g *Game) wipeDiscardLocked() uint64 {
	g.played = map[string]playedRow{}
	return g.enqueueDiscardLocked()
}

func (g *Game) burnPairLocked(kind, text string) {
	norm := normalizeCardText(text)
	if kind != kindPrompt || norm == "" {
		return
	}
	var next []burnEntry
	for _, e := range g.burns {
		if e.Kind == kind && e.Text == norm {
			continue
		}
		next = append(next, e)
	}
	g.burns = append(next, burnEntry{Kind: kind, Text: norm})
	g.enqueueBurnsLocked()
}

func (g *Game) unburnPairLocked(kind, text string) {
	norm := normalizeCardText(text)
	var next []burnEntry
	for _, e := range g.burns {
		if e.Kind == kind && e.Text == norm {
			continue
		}
		next = append(next, e)
	}
	g.burns = next
	g.enqueueBurnsLocked()
}

func (g *Game) isBurned(kind, text string) bool {
	norm := normalizeCardText(text)
	for _, e := range g.burns {
		if e.Kind == kind && e.Text == norm {
			return true
		}
	}
	return false
}

func (g *Game) lastBurns(n int) []burnEntry {
	if n <= 0 || len(g.burns) == 0 {
		return nil
	}
	if n > len(g.burns) {
		n = len(g.burns)
	}
	out := make([]burnEntry, n)
	for i := 0; i < n; i++ {
		out[i] = g.burns[len(g.burns)-1-i]
	}
	return out
}

func filterBurns(piles promptPiles, burns []burnEntry) promptPiles {
	hit := map[string]bool{}
	for _, e := range burns {
		hit[e.Kind+"\x00"+e.Text] = true
	}
	var out promptPiles
	for _, p := range piles.Prompts {
		if !hit[kindPrompt+"\x00"+normalizeCardText(p.Text)] {
			out.Prompts = append(out.Prompts, p)
		}
	}
	return out
}

func allPromptsSpent(piles promptPiles, played map[string]playedRow) bool {
	if len(piles.Prompts) == 0 {
		return false
	}
	for _, p := range piles.Prompts {
		if _, ok := played[p.key()]; !ok {
			return false
		}
	}
	return true
}

func (g *Game) pushBurnDrawerLocked(prompt playPrompt) {
	text := normalizeCardText(prompt.Text)
	if text == "" {
		return
	}
	key := kindPrompt + "\x00" + text
	var next []burnFace
	for _, f := range g.burnDrawer {
		if f.Kind+"\x00"+f.Norm == key {
			continue
		}
		next = append(next, f)
	}
	next = append(next, burnFace{
		Kind:   kindPrompt,
		Text:   prompt.Text,
		Norm:   text,
		Burned: g.isBurned(kindPrompt, prompt.Text),
	})
	if len(next) > 3 {
		next = next[len(next)-3:]
	}
	g.burnDrawer = next
}

func (g *Game) burnDrawerFacesLocked() []burnFace {
	out := make([]burnFace, len(g.burnDrawer))
	for i, f := range g.burnDrawer {
		out[i] = f
		out[i].Burned = g.isBurned(f.Kind, f.Text)
		if g.burnChecks != nil {
			out[i].Checked = g.burnChecks[f.Kind+"\x00"+f.Text]
		}
	}
	return out
}

func (g *Game) stripBurnedFromPoolLocked() {
	if g.engine == nil {
		return
	}
	pool := g.engine.PromptPool[:0]
	for _, p := range g.engine.PromptPool {
		if g.isBurned(kindPrompt, p.Text) {
			continue
		}
		pool = append(pool, p)
	}
	g.engine.PromptPool = pool
}
