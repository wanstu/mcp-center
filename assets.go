package mcpcenterapp

import (
	"embed"
	"io/fs"

	kitui "github.com/wanstu/wails-desktop-kit/ui"
)

//go:embed all:web
var embeddedFrontend embed.FS

// Assets returns the production web assets with the shared Desktop Kit UI
// mounted under /desktopkit/.
func Assets() (fs.FS, error) {
	app, err := fs.Sub(embeddedFrontend, "web")
	if err != nil {
		return nil, err
	}
	return kitui.Mount(app), nil
}
