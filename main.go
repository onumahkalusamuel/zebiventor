package main

import (
	"embed"

	"github.com/onumahkalusamuel/zebiventor/internal/controllers"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()
	general := controllers.GeneralApp()
	staff := controllers.StaffApp()
	sales := controllers.SalesApp()
	products := controllers.ProductApp()
	customers := controllers.CustomersApp()
	categories := controllers.CategoryApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:             "ZebiVentor",
		Width:             1024,
		Height:            720,
		MinWidth:          1024,
		MinHeight:         720,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Menu:             nil,
		Logger:           nil,
		LogLevel:         logger.DEBUG,
		OnDomReady:       app.domReady,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		WindowStartState: options.Normal,

		Bind: []interface{}{
			app,
			general,
			staff,
			sales,
			products,
			customers,
			categories,
		},
		// Windows platform specific options
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			ZoomFactor:           1.0,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
