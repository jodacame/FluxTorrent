// Package fluxtorrent embeds the built React UI so the binary serves it directly
// (SPEC §3 UI delivery via embed.FS).
package fluxtorrent

import (
	"embed"
	"io/fs"
)

//go:embed all:web/dist
var distEmbed embed.FS

// DistFS returns the embedded web/dist filesystem rooted at its contents.
func DistFS() fs.FS {
	sub, err := fs.Sub(distEmbed, "web/dist")
	if err != nil {
		panic(err)
	}
	return sub
}
