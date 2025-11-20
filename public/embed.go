package public

import (
	"embed"
	"io/fs"
)

//go:embed all:*
var embeddedFS embed.FS

func FS() fs.FS {
	return embeddedFS
}
