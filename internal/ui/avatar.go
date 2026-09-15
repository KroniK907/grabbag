package ui

import (
	"crypto/sha256"
	"fmt"
	"html"
	"html/template"
)

// SeedColor returns a hex fill derived from seed. Tokens and faces share it.
func SeedColor(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("#%02x%02x%02x", 80+sum[0]%140, 40+sum[1]%160, 80+sum[2]%140)
}

// AvatarSVG is a procedural face from seed. Callers store the seed, not this markup.
func AvatarSVG(seed string) template.HTML {
	sum := sha256.Sum256([]byte(seed))
	skin := SeedColor(seed)
	ink := "#1a0730"
	if sum[3]%2 == 0 {
		ink = "#fff7fb"
	}
	cx1 := 22 + int(sum[4]%8)
	cx2 := 42 - int(sum[5]%8)
	cy := 28 + int(sum[6]%6)
	r := 3 + int(sum[7]%3)
	mouthY := 44 + int(sum[8]%6)
	mouthW := 10 + int(sum[9]%10)
	hair := SeedColor(seed + "/hair")
	escaped := html.EscapeString(seed)
	markup := fmt.Sprintf(
		`<svg class="ui-avatar" viewBox="0 0 64 64" role="img" aria-label="Avatar">`+
			`<title>%s</title>`+
			`<circle cx="32" cy="32" r="30" fill="%s"/>`+
			`<path d="M8 28 Q32 %d 56 28 L54 12 Q32 0 10 12 Z" fill="%s"/>`+
			`<circle cx="%d" cy="%d" r="%d" fill="%s"/>`+
			`<circle cx="%d" cy="%d" r="%d" fill="%s"/>`+
			`<path d="M%d %d Q32 %d %d %d" fill="none" stroke="%s" stroke-width="3" stroke-linecap="round"/>`+
			`</svg>`,
		escaped,
		skin,
		8+int(sum[10]%10),
		hair,
		cx1, cy, r, ink,
		cx2, cy, r, ink,
		32-mouthW, mouthY, mouthY+int(sum[11]%8), 32+mouthW, mouthY, ink,
	)
	return template.HTML(markup)
}
