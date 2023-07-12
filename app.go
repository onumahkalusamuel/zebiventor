package main

import (
	"context"
	"fmt"

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
	// setup the database
	db.Init()
	db.InitialData()
	// add context
	a.ctx = ctx
}

func (b *App) domReady(ctx context.Context) {
	runtime.WindowMaximise(ctx)
	// Add your action here
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
