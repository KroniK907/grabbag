package host

import (
	"sort"
	"strings"
	"testing"
)

func TestFormatPlayerLine(t *testing.T) {
	t.Parallel()
	if got := formatPlayerLine(0, 0); got != "" {
		t.Fatalf("0,0 = %q", got)
	}
	if got := formatPlayerLine(3, 0); got != "Players: at least 3" {
		t.Fatalf("min only = %q", got)
	}
	if got := formatPlayerLine(0, 8); got != "Players: up to 8" {
		t.Fatalf("max only = %q", got)
	}
	if got := formatPlayerLine(2, 10); got != "Players: at least 2, up to 10" {
		t.Fatalf("both = %q", got)
	}
}

func TestCatalogSortByName(t *testing.T) {
	t.Parallel()
	type row struct {
		name string
		id   string
	}
	rows := []row{{"Zulu", "zebra"}, {"Apples", "alpha"}, {"Middling", "mid"}}
	sort.Slice(rows, func(i, j int) bool {
		an := strings.ToLower(rows[i].name)
		bn := strings.ToLower(rows[j].name)
		if an != bn {
			return an < bn
		}
		return rows[i].id < rows[j].id
	})
	if rows[0].name != "Apples" || rows[1].name != "Middling" || rows[2].name != "Zulu" {
		t.Fatalf("sort order = %#v", rows)
	}
}

func TestCatalogWarning(t *testing.T) {
	t.Parallel()
	if msg := catalogWarning(3, 0, 2); msg == "" {
		t.Fatal("expected below min warning")
	}
	if msg := catalogWarning(0, 4, 5); msg == "" {
		t.Fatal("expected above max warning")
	}
	if msg := catalogWarning(1, 8, 4); msg != "" {
		t.Fatalf("valid seated = %q", msg)
	}
}
