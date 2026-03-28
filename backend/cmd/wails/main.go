package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"com.birdhalfbaked.aml-toolkit/internal/audioout"
	"com.birdhalfbaked.aml-toolkit/internal/db"
	"com.birdhalfbaked.aml-toolkit/internal/desktop"
	"com.birdhalfbaked.aml-toolkit/internal/httpserver"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func setFrontendDistDefault() {
	if os.Getenv("AUDIO_TAGGER_FRONTEND_DIR") != "" {
		return
	}
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		return
	}
	dir := filepath.Dir(file)
	dist := filepath.Clean(filepath.Join(dir, "..", "..", "..", "frontend", "dist"))
	os.Setenv("AUDIO_TAGGER_FRONTEND_DIR", dist)
}

func main() {
	_ = os.Setenv("AUDIO_TAGGER_DESKTOP", "1")
	setFrontendDistDefault()

	dbPath, libraryDir, needOnboarding, err := desktop.PrepareDesktopPaths()
	if err != nil {
		log.Fatal(err)
	}
	_ = os.Setenv("AUDIO_TAGGER_DB", dbPath)

	stack, err := httpserver.OpenStack(httpserver.Config{
		DBPath:     dbPath,
		LibraryDir: libraryDir,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer stack.DB.Close()

	if needOnboarding {
		stack.Server.SetAPIUnlocked(false)
	}

	if err := db.RunMigrations(stack.DB); err != nil {
		log.Fatal(err)
	}

	h := httpserver.NewHandler(stack, os.Getenv("AUDIO_TAGGER_FRONTEND_DIR"))
	player := audioout.NewPlayer(stack.Repo, stack.Server)
	app := NewApp(stack, player, h)

	err = wails.Run(&options.App{
		Title:            "Audio Tagger",
		Width:            1280,
		Height:           800,
		BackgroundColour: options.NewRGBA(27, 38, 54, 1),
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		AssetServer: &assetserver.Options{
			Assets:  nil,
			Handler: h,
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
