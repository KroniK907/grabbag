package ui_test

import (
	"strings"
	"testing"

	"github.com/KroniK907/grabbag/internal/ui"
)

func TestSoftHyphenBreaksOnlyLongTokens(t *testing.T) {
	t.Parallel()
	short := ui.SoftHyphen("short")
	if short != "short" {
		t.Fatalf("short word changed: %q", short)
	}
	spaced := ui.SoftHyphen("two words")
	if strings.ContainsRune(spaced, '\u00ad') {
		t.Fatalf("spaced words got a soft hyphen: %q", spaced)
	}
	long := ui.SoftHyphen("Supercalifragilisticexpialidocious")
	parts := strings.Split(long, "\u00ad")
	if len(parts) < 2 {
		t.Fatal("long word has no soft hyphen")
	}
	for _, part := range parts {
		if len([]rune(part)) > 8 {
			t.Fatalf("chunk longer than 8: %q", part)
		}
	}
	if strings.Join(parts, "") != "Supercalifragilisticexpialidocious" {
		t.Fatalf("soft hyphens changed the letters: %q", long)
	}
}
