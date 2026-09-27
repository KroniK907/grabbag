package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
)

// result is one shot plus what the page reported.
type result struct {
	job
	probe
	// FixedText counts text nodes whose font size did not grow with the
	// browser text size, compared with the same page at 100%.
	FixedText int    `json:"fixedText,omitempty"`
	TextNodes int    `json:"textNodes,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Flagged reports whether the shot has any layout finding or error.
func (r result) Flagged() bool {
	return r.Error != "" || r.HScroll || len(r.Clipped) > 0 || len(r.Outside) > 0 || r.FixedText > 0
}

// markFixedText compares each enlarged-text shot with its 100% twin.
func markFixedText(results []result) {
	base := map[string][]float64{}
	for _, r := range results {
		if r.TextScale == 1 && r.Error == "" {
			base[r.Screen()] = r.Fonts
		}
	}
	for i := range results {
		r := &results[i]
		if r.TextScale == 1 || r.Error != "" || r.TextMode != "default-font" {
			continue
		}
		ref, ok := base[r.Screen()]
		if !ok || len(ref) != len(r.Fonts) {
			continue
		}
		r.TextNodes = len(ref)
		for k := range ref {
			if r.Fonts[k] <= ref[k]+0.01 {
				r.FixedText++
			}
		}
	}
}

type sheetRow struct {
	Page  string
	Shots []result
}

type sheetSet struct {
	Name    string
	Rows    []sheetRow
	Flagged int
	Total   int
}

func writeReport(out string, results []result) error {
	raw, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "report.json"), raw, 0o644); err != nil {
		return err
	}
	var sets []*sheetSet
	bySet := map[string]*sheetSet{}
	rows := map[string]*sheetRow{}
	for _, r := range results {
		s := bySet[r.Set]
		if s == nil {
			s = &sheetSet{Name: r.Set}
			bySet[r.Set] = s
			sets = append(sets, s)
		}
		s.Total++
		if r.Flagged() {
			s.Flagged++
		}
		key := r.Set + "|" + r.Page()
		if rows[key] == nil {
			s.Rows = append(s.Rows, sheetRow{Page: r.Page()})
			rows[key] = &s.Rows[len(s.Rows)-1]
		}
	}
	for _, s := range sets {
		for i := range s.Rows {
			rows[s.Name+"|"+s.Rows[i].Page] = &s.Rows[i]
		}
	}
	for _, r := range results {
		row := rows[r.Set+"|"+r.Page()]
		row.Shots = append(row.Shots, r)
	}
	order := map[string]int{"scenarios": 0, "sweep": 1, "devices": 2}
	sort.SliceStable(sets, func(i, j int) bool { return order[sets[i].Name] < order[sets[j].Name] })
	var buf bytes.Buffer
	if err := sheet.Execute(&buf, sets); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "index.html"), buf.Bytes(), 0o644)
}

var sheet = template.Must(template.New("sheet").Funcs(template.FuncMap{
	"caption": func(r result) string {
		s := r.Device
		if r.Landscape {
			s += " land"
		}
		if r.Players > 0 {
			s = fmt.Sprintf("%dp · %s", r.Players, s)
		}
		if r.TextScale != 1 {
			s += fmt.Sprintf(" · text %d%%", int(r.TextScale*100+0.5))
			if r.TextMode == "device" && r.UA == "ios" {
				s += " zoom"
			}
		}
		if r.Set == "scenarios" {
			s += " · " + r.Theme
		}
		return s
	},
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>GrabBag.gg UI shots</title>
<style>
  body { margin: 0; padding: 16px 20px; font: 13px/1.4 system-ui, sans-serif; background: #eceef3; color: #1b1b24; }
  h1 { font-size: 20px; margin: 0 0 8px; }
  h2 { font-size: 16px; margin: 28px 0 8px; }
  label { font-size: 13px; }
  .row { background: #fff; border-radius: 8px; padding: 8px 10px; margin: 8px 0; }
  .row h3 { font-size: 13px; margin: 0 0 6px; font-family: ui-monospace, monospace; }
  .shots { display: flex; gap: 10px; overflow-x: auto; align-items: flex-start; padding-bottom: 4px; }
  figure { margin: 0; flex: none; width: 150px; }
  figure img { width: 150px; border: 1px solid #d4d7e0; border-radius: 4px; display: block; background: #fff; }
  figure.bad img { border: 2px solid #d93025; }
  figcaption { font-size: 11px; color: #555a6a; }
  .find { color: #b3261e; font-size: 11px; }
  .stress { color: #8a5a00; font-weight: 600; }
  body.flagged-only .row:not(.has-bad) { display: none; }
  body.flagged-only figure:not(.bad) { display: none; }
</style>
</head>
<body>
<h1>GrabBag.gg UI shots</h1>
<label><input type="checkbox" onchange="document.body.classList.toggle('flagged-only', this.checked)"> Only shots with findings</label>
{{range .}}
<h2>{{.Name}} · {{.Total}} shots · {{.Flagged}} with findings</h2>
{{range .Rows}}
<div class="row{{range .Shots}}{{if .Flagged}} has-bad{{end}}{{end}}">
  <h3>{{.Page}}</h3>
  <div class="shots">
  {{range .Shots}}
    <figure class="{{if .Flagged}}bad{{end}}">
      {{if .Error}}<p class="find">{{.Error}}</p>{{else}}<a href="{{.File}}" target="_blank"><img loading="lazy" src="{{.File}}" alt=""></a>{{end}}
      <figcaption>{{if .Stress}}<span class="stress">stress </span>{{end}}{{caption .}}</figcaption>
      {{if .HScroll}}<div class="find">horizontal scroll {{.ScrollWidth}} &gt; {{.ClientWidth}}</div>{{end}}
      {{if .Outside}}<div class="find" title="{{range .Outside}}{{.}}&#10;{{end}}">{{len .Outside}} past right edge</div>{{end}}
      {{if .Clipped}}<div class="find" title="{{range .Clipped}}{{.}}&#10;{{end}}">{{len .Clipped}} clipped</div>{{end}}
      {{if .FixedText}}<div class="find">{{.FixedText}}/{{.TextNodes}} text did not scale</div>{{end}}
      {{if .SmallTargets}}<div class="meta" title="{{range .SmallTargets}}{{.}}&#10;{{end}}">{{len .SmallTargets}} small tap targets</div>{{end}}
    </figure>
  {{end}}
  </div>
</div>
{{end}}
{{end}}
</body>
</html>
`))
