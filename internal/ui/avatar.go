package ui

import (
	"crypto/sha256"
	"fmt"
	"html"
	"html/template"
	"math"
	"strconv"
)

// TokenColor is the player's seat color. It stays with them across screens.
func TokenColor(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	h := int(sum[0]) * 360 / 256
	s := 68 + int(sum[1])%18
	l := 36 + int(sum[2])%14
	return hslToHex(h, s, l)
}

// SeedColor is the token color. Kept so templates and older call sites match.
func SeedColor(seed string) string {
	return TokenColor(seed)
}

var skinTones = []string{
	"#f7d3b8",
	"#efc09a",
	"#e0a070",
	"#c47a48",
	"#8d5524",
	"#5c3310",
}

var hairTones = []string{
	"#1b130c",
	"#3b2214",
	"#6b3a1f",
	"#c48a2a",
	"#2a1a16",
	"#4a2f23",
}

// FaceColor is a skin tone from the seed. It is not the token color, so the
// face stays readable on the player's circle.
func FaceColor(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return skinTones[int(sum[3])%len(skinTones)]
}

// AvatarSVG is a procedural face from seed. Callers store the seed, not this markup.
func AvatarSVG(seed string) template.HTML {
	sum := sha256.Sum256([]byte(seed))
	skin := FaceColor(seed)
	hair := hairTones[int(sum[4])%len(hairTones)]
	ink := "#1a0730"
	if hexLuma(skin) < 0.42 {
		ink = "#fff4e8"
	}
	cx1 := 24 + int(sum[6]%5)
	cx2 := 40 - int(sum[7]%5)
	cy := 33 + int(sum[8]%4)
	r := 2 + int(sum[9]%2)
	mouthY := 46 + int(sum[10]%4)
	smile := 2 + int(sum[11]%5)
	escaped := html.EscapeString(seed)
	var b string
	b += fmt.Sprintf(`<svg class="ui-avatar" viewBox="0 0 64 64" role="img" aria-label="Avatar"><title>%s</title>`, escaped)
	switch sum[5] % 3 {
	case 0:
		b += fmt.Sprintf(`<ellipse cx="32" cy="24" rx="19" ry="14" fill="%s"/>`, hair)
	case 1:
		b += fmt.Sprintf(
			`<circle cx="17" cy="30" r="11" fill="%s"/><circle cx="47" cy="30" r="11" fill="%s"/><ellipse cx="32" cy="20" rx="17" ry="13" fill="%s"/>`,
			hair, hair, hair,
		)
	}
	b += fmt.Sprintf(`<circle cx="32" cy="36" r="18" fill="%s"/>`, skin)
	b += fmt.Sprintf(
		`<circle cx="%d" cy="%d" r="%d" fill="%s"/>`+
			`<circle cx="%d" cy="%d" r="%d" fill="%s"/>`+
			`<path d="M%d %d Q32 %d %d %d" fill="none" stroke="%s" stroke-width="2.5" stroke-linecap="round"/>`+
			`</svg>`,
		cx1, cy, r, ink,
		cx2, cy, r, ink,
		26, mouthY, mouthY+smile, 38, mouthY, ink,
	)
	return template.HTML(b)
}

func hslToHex(h, s, l int) string {
	hf := float64(h)
	sf := float64(s) / 100
	lf := float64(l) / 100
	c := (1 - math.Abs(2*lf-1)) * sf
	x := c * (1 - math.Abs(math.Mod(hf/60, 2)-1))
	m := lf - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return fmt.Sprintf("#%02x%02x%02x", toByte(r+m), toByte(g+m), toByte(b+m))
}

func toByte(v float64) int {
	n := int(math.Round(v * 255))
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

func hexLuma(hex string) float64 {
	if len(hex) != 7 || hex[0] != '#' {
		return 0.5
	}
	r, err1 := strconv.ParseUint(hex[1:3], 16, 8)
	g, err2 := strconv.ParseUint(hex[3:5], 16, 8)
	b, err3 := strconv.ParseUint(hex[5:7], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0.5
	}
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255
}
