package main

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/KroniK907/grabbag/internal/host"
)

// job is one screenshot.
type job struct {
	Set       string  `json:"set"`
	Package   string  `json:"package"`
	Surface   string  `json:"surface"`
	Name      string  `json:"name"`
	Viewer    string  `json:"viewer"`
	Players   int     `json:"players,omitempty"`
	Stress    bool    `json:"stress,omitempty"`
	Theme     string  `json:"theme"`
	Device    string  `json:"device"`
	Landscape bool    `json:"landscape,omitempty"`
	TextScale float64 `json:"textScale"`
	TextMode  string  `json:"textMode,omitempty"`
	Width     int     `json:"width"`
	Height    int     `json:"height"`
	Scale     float64 `json:"scale"`
	Mobile    bool    `json:"mobile"`
	Tablet    bool    `json:"tablet,omitempty"`
	UA        string  `json:"ua,omitempty"`
	Path      string  `json:"path"`
	File      string  `json:"file"`
}

// Page is the scenario key shared by every shot of one page.
func (j job) Page() string { return j.Package + "/" + j.Surface + "/" + j.Name }

// Screen is the device key used to pair a text-size shot with its 100% twin.
func (j job) Screen() string {
	return fmt.Sprintf("%s|%s|%d|%s|%s|%t", j.Set, j.Page(), j.Players, j.Theme, j.Device, j.Landscape)
}

func (j job) URL(base string) string {
	q := url.Values{"theme": {j.Theme}}
	if j.Players > 0 {
		q.Set("players", strconv.Itoa(j.Players))
	}
	return base + j.Path + "?" + q.Encode()
}

func matches(only []string, key string) bool {
	if len(only) == 0 {
		return true
	}
	for _, pattern := range only {
		if ok, _ := path.Match(pattern, key); ok {
			return true
		}
	}
	return false
}

// plan expands the index into shots for the chosen sets.
func plan(cfg config, index host.PreviewIndex) []job {
	var jobs []job
	add := func(set string, s host.PreviewScenario, players int, theme string, d host.PreviewDevice, landscape bool, text float64) {
		j := job{
			Set: set, Package: s.Package, Surface: s.Surface, Name: s.Name, Viewer: s.Viewer,
			Players: players, Stress: players > s.RealisticPlayers && s.RealisticPlayers > 0,
			Theme: theme, Device: d.ID, Landscape: landscape, TextScale: text,
			Width: d.Width, Height: d.Height, Scale: d.Scale, Mobile: d.Mobile, Tablet: d.Tablet, UA: d.UA, Path: s.Path,
		}
		if landscape {
			j.Width, j.Height = d.Height, d.Width
		}
		if text != 1 {
			j.TextMode = cfg.textMode
		}
		if cfg.maxDPR > 0 && j.Scale > cfg.maxDPR {
			j.Scale = cfg.maxDPR
		}
		name := strings.NewReplacer("~", "--").Replace(s.Name)
		file := fmt.Sprintf("%s-%s", name, theme)
		if players > 0 {
			file = fmt.Sprintf("%s@%dp-%s", name, players, theme)
		}
		file += "-" + d.ID
		if landscape {
			file += "-land"
		}
		if text != 1 {
			file += fmt.Sprintf("-t%d", int(text*100+0.5))
		}
		j.File = path.Join(set, s.Package, s.Surface, file+".png")
		jobs = append(jobs, j)
	}
	for _, s := range index.Scenarios {
		if !matches(cfg.only, s.Package+"/"+s.Surface+"/"+s.Name) {
			continue
		}
		frame := index.Frames[s.Frame]
		if cfg.sets["scenarios"] {
			for _, theme := range cfg.themes {
				add("scenarios", s, s.DefaultPlayers, theme, frame, false, 1)
			}
		}
		if cfg.sets["sweep"] && !strings.Contains(s.Name, "~") {
			for _, n := range s.Players {
				add("sweep", s, n, cfg.themes[0], frame, false, 1)
			}
		}
		if cfg.sets["devices"] && s.Sample {
			for _, d := range index.Devices {
				for _, text := range index.TextScales {
					add("devices", s, s.RealisticPlayers, cfg.themes[0], d, false, text)
					if d.Tablet {
						add("devices", s, s.RealisticPlayers, cfg.themes[0], d, true, text)
					}
				}
			}
		}
	}
	return jobs
}
