package ui

// Neon cabinet theme ids. Pages set html[data-theme]. Token values live in live.css.
const (
	// ThemeNeonLight is the same shapes with the light token set.
	ThemeNeonLight = "neon-light"
	// ThemeNeonDark is the host default.
	ThemeNeonDark = "neon-dark"
)

// DefaultTheme is the theme new pages render until /settings can flip it.
const DefaultTheme = ThemeNeonDark

// Chrome is the shared document shell. Pages pass it into ui-start templates.
type Chrome struct {
	Title         string
	Theme         string
	NoticeTargets string
	// Static renders a frozen page for UI previews: no htmx, no SSE, no
	// heartbeat, and timers show their server-rendered value.
	Static bool
	// OpenIDs lists element ids a static preview opens on load (drawers get
	// is-open, hidden modals are shown, details are expanded).
	OpenIDs string
}

// Page returns dark neon chrome, the default.
func Page(title string) Chrome {
	return Chrome{Title: title, Theme: DefaultTheme}
}

// NormalizeTheme maps unknown values to the dark default.
func NormalizeTheme(value string) string {
	if value == ThemeNeonLight {
		return ThemeNeonLight
	}
	return ThemeNeonDark
}
