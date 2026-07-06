//go:build !terminal

package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	exe, _ := os.Executable()
	os.Chdir(filepath.Dir(exe))
	runSetup()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "MCChecker",
		Width:     1100,
		Height:    750,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Logger:             logger.NewDefaultLogger(),
		LogLevel:           logger.WARNING,
		LogLevelProduction: logger.ERROR,
		OnStartup:          app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
