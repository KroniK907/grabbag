// Command grabbag-uishots screenshots every UI preview scenario in headless
// Chromium. It serves the host preview gallery in-process (no store, no
// data dir) unless -base points at a host already running -dev-preview.
//
// Sets:
//
//	scenarios  every scenario at its default table size, both themes
//	sweep      every player-count scenario at min, middle, 12, and max
//	devices    Sample scenarios at the realistic max on every phone and
//	           tablet (tablets in both orientations) at every text size
//
// Text sizes follow -text-mode. In device mode an Android shot scales every
// font size, as the OS font-size setting does in Chrome, and an iOS shot
// zooms the page (a narrower CSS viewport), as Safari's aA menu does. In
// default-font mode only the browser default font size grows, and the report
// counts text that stayed the same size (fixed px sizes).
//
// Output is PNGs, report.json with layout findings, and index.html, a
// contact sheet with one row per page.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/KroniK907/grabbag/internal/host"
)

type config struct {
	out     string
	sets    map[string]bool
	only    []string
	themes  []string
	chrome  string
	workers int
	maxDPR  float64
	base    string
	serve   string
	timeout time.Duration
	// textMode is "device" or "default-font"; see the -text-mode flag.
	textMode string
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if cfg.serve != "" {
		log.Printf("UI preview http://%s%s", cfg.serve, host.PreviewPath)
		log.Fatal(http.ListenAndServe(cfg.serve, host.PreviewHandler()))
	}
	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func parseFlags(args []string) (config, error) {
	fs := flag.NewFlagSet("grabbag-uishots", flag.ContinueOnError)
	out := fs.String("out", "shots", "output directory")
	sets := fs.String("set", "scenarios,sweep,devices", "comma list of sets: scenarios, sweep, devices")
	only := fs.String("only", "", "comma list of package/surface/name globs, such as apples/phone/* or lobby/*/*")
	themes := fs.String("themes", "neon-dark,neon-light", "themes for the scenarios set; sweep and devices use the first")
	chrome := fs.String("chrome", "", "Chromium or Chrome binary; empty finds chromium-browser, chromium, or google-chrome")
	workers := fs.Int("workers", 4, "parallel browser tabs")
	maxDPR := fs.Float64("max-dpr", 2, "cap on device pixel ratio, to keep PNGs small; 0 uses each device's own")
	base := fs.String("base", "", "base URL of a host running -dev-preview; empty serves in-process")
	serve := fs.String("serve", "", "only serve the gallery at this address, such as 127.0.0.1:8655")
	timeout := fs.Duration("timeout", 20*time.Second, "per-page timeout")
	textMode := fs.String("text-mode", "device", "device: Android scales every font like the OS font size, iOS zooms the page like Safari aA; default-font: only raise the browser default font size and report text that ignores it")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg := config{
		out: *out, chrome: *chrome, workers: max(1, *workers), maxDPR: *maxDPR,
		base: strings.TrimRight(*base, "/"), serve: *serve, timeout: *timeout, sets: map[string]bool{},
		textMode: *textMode,
	}
	if cfg.textMode != "device" && cfg.textMode != "default-font" {
		return config{}, fmt.Errorf("grabbag-uishots: -text-mode must be device or default-font, got %q", cfg.textMode)
	}
	for _, s := range splitList(*sets) {
		switch s {
		case "scenarios", "sweep", "devices":
			cfg.sets[s] = true
		default:
			return config{}, fmt.Errorf("grabbag-uishots: unknown set %q", s)
		}
	}
	cfg.only = splitList(*only)
	for _, pattern := range cfg.only {
		if _, err := path.Match(pattern, ""); err != nil {
			return config{}, fmt.Errorf("grabbag-uishots: bad -only pattern %q", pattern)
		}
	}
	cfg.themes = splitList(*themes)
	if len(cfg.themes) == 0 {
		return config{}, errors.New("grabbag-uishots: -themes is empty")
	}
	return cfg, nil
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func run(cfg config) error {
	base := cfg.base
	if base == "" {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
		srv := &http.Server{Handler: host.PreviewHandler()}
		go func() { _ = srv.Serve(ln) }()
		defer srv.Close()
		base = "http://" + ln.Addr().String()
	}
	index, err := fetchIndex(base)
	if err != nil {
		return err
	}
	jobs := plan(cfg, index)
	if len(jobs) == 0 {
		return errors.New("grabbag-uishots: no scenarios match")
	}
	if err := os.MkdirAll(cfg.out, 0o755); err != nil {
		return err
	}
	log.Printf("%d screenshots into %s", len(jobs), cfg.out)
	start := time.Now()
	results, err := shoot(context.Background(), cfg, base, jobs)
	if err != nil {
		return err
	}
	markFixedText(results)
	if err := writeReport(cfg.out, results); err != nil {
		return err
	}
	flagged := 0
	for _, r := range results {
		if r.Flagged() {
			flagged++
		}
	}
	log.Printf("done in %s: %d shots, %d with findings; open %s", time.Since(start).Round(time.Second), len(results), flagged, path.Join(cfg.out, "index.html"))
	return nil
}

func fetchIndex(base string) (host.PreviewIndex, error) {
	var doc host.PreviewIndex
	resp, err := http.Get(base + host.PreviewPath + "index.json")
	if err != nil {
		return doc, fmt.Errorf("grabbag-uishots: read index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return doc, fmt.Errorf("grabbag-uishots: index returned %s; is -dev-preview on?", resp.Status)
	}
	return doc, json.NewDecoder(resp.Body).Decode(&doc)
}
