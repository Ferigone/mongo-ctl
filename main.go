package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"mongoctl/internal/app"
	"mongoctl/internal/elevate"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Administrator-only work runs by re-launching this same executable with a
	// flag rather than shipping a separate helper, which keeps the application a
	// single file. That mode does no UI work and exits immediately.
	if elevate.Requested() {
		os.Exit(elevate.Execute())
	}

	core, err := app.NewCore()
	if err != nil {
		log.Fatalf("MongoCtl could not start: %v", err)
	}

	err = wails.Run(&options.App{
		Title:     "MongoCtl",
		Width:     1360,
		Height:    880,
		MinWidth:  1024,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 12, G: 14, B: 19, A: 1},
		OnStartup:        core.Startup,
		OnBeforeClose:    core.Shutdown,
		Bind: []any{
			app.NewInstanceService(core),
			app.NewClusterService(core),
			app.NewSystemService(core),
		},
	})
	if err != nil {
		log.Fatalf("MongoCtl exited with an error: %v", err)
	}
}
