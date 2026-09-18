package host

import "testing"

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
