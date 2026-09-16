// Package docs embeds the operator HTML manual from this directory.
package docs

import "embed"

// Files is the HTML tree. index.html is the how-to-run guide.
// game-contract.html is the human Game/Helper API reference.
//
//go:embed *.html
var Files embed.FS
