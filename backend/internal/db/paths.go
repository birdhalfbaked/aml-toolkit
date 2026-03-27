package db

import (
	"os"
	"path/filepath"
)

// DefaultDataDir matches main: AUDIO_TAGGER_DATA or "data".
func DefaultDataDir() string {
	d := os.Getenv("AUDIO_TAGGER_DATA")
	if d == "" {
		return "data"
	}
	return d
}

// DefaultDBPath matches main: AUDIO_TAGGER_DB or <dataDir>/app.db.
func DefaultDBPath() string {
	if p := os.Getenv("AUDIO_TAGGER_DB"); p != "" {
		return p
	}
	return filepath.Join(DefaultDataDir(), "app.db")
}
