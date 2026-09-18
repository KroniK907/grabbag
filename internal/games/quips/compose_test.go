package quips

import "testing"

func TestComposeLockBannedAndDuplicate(t *testing.T) {
	t.Parallel()
	p := composePolicy{Cap: 40, Banned: "bad", ShowMatchedWord: true}
	issues := p.lockIssues([]string{"hello bad word"}, []string{})
	if len(issues) != 1 || issues[0].Banned == "" {
		t.Fatalf("issues=%v", issues)
	}
	issues = p.lockIssues([]string{"unique"}, []string{normalizeCardText("unique")})
	if len(issues) != 1 || !issues[0].Dup {
		t.Fatalf("dup issues=%v", issues)
	}
}

func TestComposeTimerSubmit(t *testing.T) {
	t.Parallel()
	p := composePolicy{Cap: 20, Banned: "nope"}
	if got := p.timerSubmit("  fine  ", []string{normalizeCardText("fine")}); got != "" {
		t.Fatalf("timer should blank dup, got %q", got)
	}
	if got := p.timerSubmit("kept", nil); got != "kept" {
		t.Fatalf("timer keep got %q", got)
	}
	if got := p.timerSubmit("has nope inside", nil); got != "" {
		t.Fatalf("timer banned got %q", got)
	}
}
