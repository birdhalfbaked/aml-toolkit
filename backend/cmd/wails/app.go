package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"com.birdhalfbaked.aml-toolkit/internal/audioout"
	"com.birdhalfbaked.aml-toolkit/internal/dataset"
	"com.birdhalfbaked.aml-toolkit/internal/httpserver"
	"com.birdhalfbaked.aml-toolkit/internal/models"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// APIDispatchResult is the Wails-visible shape of [App.ApiDispatch] (multi-return is not mapped to JS arrays reliably).
type APIDispatchResult struct {
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Body        []byte `json:"body"`
}

// DesktopAudioFileBytes is raw audio for WaveSurfer loaded in-process on desktop.
type DesktopAudioFileBytes struct {
	Data []byte `json:"data"`
	Mime string `json:"mime"`
}

// App is bound to the frontend for desktop-native audio playback (gopxl/beep).
// HTTP API and SPA are served through Wails [assetserver.Options.Handler].
type App struct {
	ctx        context.Context
	stack      *httpserver.Stack
	player     *audioout.Player
	apiHandler http.Handler
}

func NewApp(stack *httpserver.Stack, player *audioout.Player, apiHandler http.Handler) *App {
	return &App{stack: stack, player: player, apiHandler: apiHandler}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := audioout.WarmSpeaker(); err != nil {
		log.Printf("audio-tagger: native speaker init failed: %v", err)
	}
	// Desktop playback is entirely in Go (beep/speaker). The UI only subscribes to this event for playhead sync.
	a.player.OnPosition = func(ms int64) {
		runtime.EventsEmit(a.ctx, "desktop:audio:position", ms)
	}
}

func (a *App) shutdown(ctx context.Context) {
	_ = ctx
	a.player.Stop()
}

// DesktopAudioPlay starts system playback for a stored audio file (wav/mp3) at startMs from the file start.
func (a *App) DesktopAudioPlay(fileID int64, startMs int64) error {
	return a.player.Play(context.Background(), fileID, startMs)
}

// DesktopAudioStop stops system playback.
func (a *App) DesktopAudioStop() {
	a.player.Stop()
}

// DesktopAudioSeekMs seeks the current desktop stream.
func (a *App) DesktopAudioSeekMs(ms int64) error {
	return a.player.SeekToMs(ms)
}

// DesktopAudioPause pauses speaker output (see beep/speaker.Lock).
func (a *App) DesktopAudioPause() {
	a.player.Pause()
}

// DesktopAudioResume resumes after [DesktopAudioPause].
func (a *App) DesktopAudioResume() {
	a.player.Resume()
}

// ApiDispatch runs an HTTP request through the same handler as the asset server (in-process REST for the desktop UI).
func (a *App) ApiDispatch(method, path, contentType string, body []byte) (*APIDispatchResult, error) {
	if a.apiHandler == nil {
		return nil, errors.New("api handler not configured")
	}
	u, err := url.Parse("http://127.0.0.1" + path)
	if err != nil {
		return nil, err
	}
	var rdr io.Reader = http.NoBody
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, u.String(), rdr)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	a.apiHandler.ServeHTTP(rec, req)
	return &APIDispatchResult{
		Status:      rec.Code,
		ContentType: rec.Header().Get("Content-Type"),
		Body:        rec.Body.Bytes(),
	}, nil
}

// DesktopReadAudioFileForWaveform returns raw audio bytes for WaveSurfer when GET /api/files/:id/audio is unreliable in WebView2.
func (a *App) DesktopReadAudioFileForWaveform(fileID int64) (*DesktopAudioFileBytes, error) {
	data, mime, err := a.stack.Server.ReadAudioFileBytes(context.Background(), fileID)
	if err != nil {
		return nil, err
	}
	return &DesktopAudioFileBytes{Data: data, Mime: mime}, nil
}

// DesktopImportFromPaths imports absolute filesystem paths in Go (ZIP / WAV / MP3).
// Used for native file dialog and Wails OnFileDrop; avoids WebView multipart upload.
func (a *App) DesktopImportFromPaths(collectionID int64, paths []string) ([]models.AudioFile, error) {
	if len(paths) == 0 {
		return []models.AudioFile{}, nil
	}
	return a.stack.Server.ImportFilesFromPaths(context.Background(), collectionID, paths)
}

// DesktopPickAndImportFiles opens an OS file dialog then calls [DesktopImportFromPaths].
func (a *App) DesktopPickAndImportFiles(collectionID int64) ([]models.AudioFile, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import ZIP or audio files",
		Filters: []runtime.FileFilter{
			{DisplayName: "ZIP or audio (*.zip, *.wav, *.mp3)", Pattern: "*.zip;*.ZIP;*.wav;*.WAV;*.mp3;*.MP3"},
		},
	})
	if err != nil {
		return nil, err
	}
	return a.DesktopImportFromPaths(collectionID, paths)
}

// DesktopOpenLibraryFolder opens an OS directory picker for the welcome / library path step.
func (a *App) DesktopOpenLibraryFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose folder for projects and audio files",
	})
}

// DesktopExportDatasetZip prompts for a save path and writes the dataset folder as a zip.
// Returns the saved file path, or empty string if the user cancelled (no error).
func (a *App) DesktopExportDatasetZip(datasetID int64) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "Export dataset as ZIP",
		DefaultFilename:      fmt.Sprintf("dataset_%d.zip", datasetID),
		Filters:              []runtime.FileFilter{{DisplayName: "ZIP archive", Pattern: "*.zip"}},
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	ds, err := a.stack.Repo.GetDataset(context.Background(), datasetID)
	if err != nil {
		return "", err
	}
	if ds == nil {
		return "", errors.New("dataset not found")
	}
	if ds.StorageRoot == "" {
		return "", errors.New("dataset has no files on disk")
	}
	if err := dataset.ExportZipToPath(ds.StorageRoot, path); err != nil {
		return "", err
	}
	return path, nil
}
