package main

import (
	"context"
	"fmt"
	"os"

	"github.com/onumahkalusamuel/zebiventor/config"
	"github.com/onumahkalusamuel/zebiventor/internal/controllers"
	"github.com/onumahkalusamuel/zebiventor/internal/db"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	// add context
	a.ctx = ctx
	// create folders
	os.Mkdir(config.AppDataFolder, 0755)
	os.Mkdir(config.FilesFolder, 0755)
	// setup the database
	db.Init()
	db.InitialData()
	var gen = controllers.GeneralApp()
	details := gen.StoreDetails()
	runtime.WindowSetTitle(a.ctx, fmt.Sprintf("%v - %v - %v", details["name"].(string), config.AppName, config.AppVersion))
}

func (b *App) domReady(ctx context.Context) {
	runtime.WindowMaximise(ctx)
	// Add your action here
}

// beforeClose is called when the application is about to quit,
// either by clicking the window close button or calling runtime.Quit.
// Returning true will cause the application to continue, false will continue shutdown as normal.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	// Perform your teardown here
}
