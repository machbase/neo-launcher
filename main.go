package main

import (
	"embed"
	"net/http"
	"strings"

	"github.com/machbase/neo-launcher/backend"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := backend.NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "neo-launcher",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: NewFileLoader(),
		},
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

//go:embed all:frontend/src/assets/static/**
var staticFiles embed.FS

type FileLoader struct {
	http.Handler
	fs http.Handler
}

func NewFileLoader() *FileLoader {
	return &FileLoader{fs: http.FileServer(http.FS(staticFiles))}
}

func (h *FileLoader) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	rewrite := req.URL.Path
	if strings.HasPrefix(rewrite, "/static/") {
		rewrite = "frontend/src/assets" + rewrite
	}
	req.URL.Path = rewrite
	h.fs.ServeHTTP(res, req)
}
