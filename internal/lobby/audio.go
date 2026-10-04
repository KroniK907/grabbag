package lobby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Board audio layers. Master scales the other two.
const (
	AudioMaster  = "master"
	AudioMusic   = "music"
	AudioEffects = "effects"
)

// Board sound states the board shell reports through POST /board/audio.
const (
	AudioUnlocked  = "unlocked"
	AudioWaiting   = "waiting"
	AudioDismissed = "dismissed"
)

// AudioLevel is one mixer row: volume 0 to 100 and mute.
type AudioLevel struct {
	Volume int  `json:"volume"`
	Muted  bool `json:"muted"`
}

// AudioLevels is the host-wide board mixer the operator sets on /settings.
type AudioLevels struct {
	Master  AudioLevel `json:"master"`
	Music   AudioLevel `json:"music"`
	Effects AudioLevel `json:"effects"`
}

// DefaultAudioLevels are the mixer levels before the operator changes them.
var DefaultAudioLevels = AudioLevels{
	Master:  AudioLevel{Volume: 80},
	Music:   AudioLevel{Volume: 60},
	Effects: AudioLevel{Volume: 75},
}

// audioRow is one /settings Audio row.
type audioRow struct {
	Layer string
	Label string
	AudioLevel
	Test bool
}

func (levels AudioLevels) rows() []audioRow {
	return []audioRow{
		{Layer: AudioMaster, Label: "Master", AudioLevel: levels.Master},
		{Layer: AudioMusic, Label: "Music", AudioLevel: levels.Music, Test: true},
		{Layer: AudioEffects, Label: "Effects", AudioLevel: levels.Effects, Test: true},
	}
}

// audioView is the /settings Audio group.
type audioView struct {
	Rows   []audioRow
	Status string
}

func audioColumns(layer string) (volume, muted string, ok bool) {
	switch layer {
	case AudioMaster, AudioMusic, AudioEffects:
		return "audio_" + layer + "_volume", "audio_" + layer + "_muted", true
	}
	return "", "", false
}

// AudioLevels reads the stored board mixer.
func (l *Lobby) AudioLevels(ctx context.Context) (AudioLevels, error) {
	var levels AudioLevels
	var mm, mu, me int
	err := l.sql.QueryRowContext(ctx, `
SELECT audio_master_volume, audio_master_muted,
       audio_music_volume, audio_music_muted,
       audio_effects_volume, audio_effects_muted
FROM room_state WHERE id = 1`).Scan(
		&levels.Master.Volume, &mm,
		&levels.Music.Volume, &mu,
		&levels.Effects.Volume, &me,
	)
	if err != nil {
		return AudioLevels{}, fmt.Errorf("lobby: read audio levels: %w", err)
	}
	levels.Master.Muted = mm != 0
	levels.Music.Muted = mu != 0
	levels.Effects.Muted = me != 0
	return levels, nil
}

// AudioLevelsJSON is the board mixer as the JSON the board shell reads. A
// failed read gives the defaults, so the board still plays.
func (l *Lobby) AudioLevelsJSON(ctx context.Context) string {
	levels, err := l.AudioLevels(ctx)
	if err != nil {
		levels = DefaultAudioLevels
	}
	b, _ := json.Marshal(levels)
	return string(b)
}

// AudioStatus is the latest board sound state, or "" when no board has
// reported since the host started. It lives in memory only.
func (l *Lobby) AudioStatus() string {
	l.audioMu.Lock()
	defer l.audioMu.Unlock()
	return l.audioStatus
}

// setAudio saves one mixer row from /settings. The form carries layer plus
// volume, muted, or both. Every board hears the new levels at once.
func (l *Lobby) setAudio(w http.ResponseWriter, r *http.Request) {
	if !l.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	volumeCol, mutedCol, ok := audioColumns(r.PostFormValue("layer"))
	if !ok {
		http.Error(w, "Unknown audio layer.", http.StatusBadRequest)
		return
	}
	var sets []string
	var args []any
	if v := strings.TrimSpace(r.PostFormValue("volume")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 100 {
			http.Error(w, "Volume must be 0 to 100.", http.StatusBadRequest)
			return
		}
		sets = append(sets, volumeCol+" = ?")
		args = append(args, n)
	}
	if v := r.PostFormValue("muted"); v != "" {
		sets = append(sets, mutedCol+" = ?")
		args = append(args, boolToInt(v == "1" || v == "on"))
	}
	if len(sets) == 0 {
		http.Error(w, "Nothing to change.", http.StatusBadRequest)
		return
	}
	if _, err := l.sql.ExecContext(r.Context(), `UPDATE room_state SET `+strings.Join(sets, ", ")+` WHERE id = 1`, args...); err != nil {
		http.Error(w, "Could not save the audio level.", http.StatusInternalServerError)
		return
	}
	l.events.PublishData("audio", l.AudioLevelsJSON(r.Context()))
	l.writeAudio(w, r)
}

// testAudio plays the short sample for one layer on every board.
func (l *Lobby) testAudio(w http.ResponseWriter, r *http.Request) {
	if !l.requireAdmin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	layer := r.PostFormValue("layer")
	if layer != AudioMusic && layer != AudioEffects {
		http.Error(w, "Unknown audio layer.", http.StatusBadRequest)
		return
	}
	l.events.PublishData("audio-test", layer)
	if hxRequest(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// boardAudio records the board's sound state. The TV is usually not signed
// in, so this needs no admin session. It keeps only the latest report.
func (l *Lobby) boardAudio(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read the form.", http.StatusBadRequest)
		return
	}
	state := r.PostFormValue("state")
	switch state {
	case AudioUnlocked, AudioWaiting, AudioDismissed:
	default:
		http.Error(w, "Unknown audio state.", http.StatusBadRequest)
		return
	}
	l.audioMu.Lock()
	changed := l.audioStatus != state
	l.audioStatus = state
	l.audioMu.Unlock()
	if changed {
		l.events.Publish("audio-status")
	}
	w.WriteHeader(http.StatusNoContent)
}

func (l *Lobby) audioStatusPartial(w http.ResponseWriter, r *http.Request) {
	if !l.requireAdmin(w, r) {
		return
	}
	l.render(w, "settings-audio-status", audioView{Status: l.AudioStatus()}, http.StatusOK)
}

func (l *Lobby) audioView(ctx context.Context) (audioView, error) {
	levels, err := l.AudioLevels(ctx)
	if err != nil {
		return audioView{}, err
	}
	return audioView{Rows: levels.rows(), Status: l.AudioStatus()}, nil
}

// writeAudio ends a mixer write. A mute toggle swaps the Audio group; a
// slider gets 204 so the thumb under the operator's finger is left alone.
func (l *Lobby) writeAudio(w http.ResponseWriter, r *http.Request) {
	if !hxRequest(r) {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if strings.TrimPrefix(r.Header.Get("HX-Target"), "#") != "settings-audio" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	view, err := l.audioView(r.Context())
	if err != nil {
		http.Error(w, "Could not read the audio levels.", http.StatusInternalServerError)
		return
	}
	l.render(w, "settings-audio", view, http.StatusOK)
}
