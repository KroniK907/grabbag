package ui_test

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/KroniK907/grabbag/internal/ui"
)

func TestScenarioSweep(t *testing.T) {
	t.Parallel()
	cases := []struct {
		min, max int
		want     []int
	}{
		{0, 0, nil},
		{1, 0, []int{1, 7, 12, 64}},
		{2, 0, []int{2, 7, 12, 64}},
		{2, 8, []int{2, 5, 8}},
		{3, 3, []int{3}},
	}
	for _, c := range cases {
		got := ui.Scenario{MinPlayers: c.min, MaxPlayers: c.max}.Sweep()
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Sweep(%d,%d) = %v, want %v", c.min, c.max, got, c.want)
		}
	}
	s := ui.Scenario{MinPlayers: 2}
	if s.ClampPlayers(0) != 7 || s.ClampPlayers(1) != 2 || s.ClampPlayers(99) != 64 || s.RealisticPlayers() != 12 {
		t.Fatalf("clamp/realistic wrong: %d %d %d %d", s.ClampPlayers(0), s.ClampPlayers(1), s.ClampPlayers(99), s.RealisticPlayers())
	}
}

type previewView struct {
	ui.Chrome
	Name  string
	Items []string
	Inner struct{ A, B int }
}

func TestPatchViewStampsChromeAndMerges(t *testing.T) {
	t.Parallel()
	view := previewView{Name: "before", Items: []string{"x"}}
	view.Inner.A, view.Inner.B = 1, 2
	got, err := ui.PatchView(view, ui.Preview{
		Theme: ui.ThemeNeonDark, Open: "drawer",
		Patches: [][]byte{[]byte(`{"Name":"after","Inner":{"B":5}}`), []byte(`{"Items":["y","z"]}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	v := got.(previewView)
	if !v.Static || v.Theme != ui.ThemeNeonDark || v.OpenIDs != "drawer" {
		t.Fatalf("chrome = %+v", v.Chrome)
	}
	if v.Name != "after" || v.Inner.A != 1 || v.Inner.B != 5 || len(v.Items) != 2 {
		t.Fatalf("patched = %+v", v)
	}
	if _, err := ui.PatchView(view, ui.Preview{Patches: [][]byte{[]byte(`{"Name":7}`)}}); err == nil {
		t.Fatal("a patch that does not fit the view should fail")
	}
}

func TestRenderScenarioIsStatic(t *testing.T) {
	t.Parallel()
	tmpl := ui.MustParse(fstest.MapFS{"p.html": {Data: []byte(`{{template "ui-start" .}}<p>{{.Name}}</p>{{template "ui-end"}}`)}}, "*.html")
	var buf bytes.Buffer
	if err := ui.RenderScenario(&buf, tmpl, "p.html", previewView{Name: "hi"}, ui.Preview{}); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	if !strings.Contains(body, "window.grabbagStatic") || strings.Contains(body, "sse-connect") || strings.Contains(body, "htmx.min.js") {
		t.Fatalf("static page still wires live updates:\n%s", body)
	}
}

func TestWithVariants(t *testing.T) {
	t.Parallel()
	render := func(io.Writer, ui.Preview) error { return nil }
	list := []ui.Scenario{{Surface: "board", Name: "draw", Render: render}}
	files := fstest.MapFS{"board.draw.paused.json": {Data: []byte(`{"Paused":true}`)}}
	got, err := ui.WithVariants(list, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Name != "draw~paused" || string(got[1].Patches[0]) != `{"Paused":true}` {
		t.Fatalf("variants = %+v", got)
	}
	if _, err := ui.WithVariants(list, fstest.MapFS{"board.gone.x.json": {Data: []byte(`{}`)}}); err == nil {
		t.Fatal("orphan variant should fail")
	}
}
