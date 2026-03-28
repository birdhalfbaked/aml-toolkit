package desktop

import (
	"os"
	"path/filepath"
)

const appDirName = "audio-tagger"

// ApplyDataEnvDefaults sets AUDIO_TAGGER_DB to a file under the OS user config directory when unset,
// so the desktop build keeps SQLite out of the working directory. Library/audio paths are separate
// (see [PrepareDesktopPaths] and config.json).
func ApplyDataEnvDefaults() {
	if os.Getenv("AUDIO_TAGGER_DB") != "" {
		return
	}
	st, err := StateDir()
	if err != nil {
		return
	}
	dbDir := filepath.Join(st, "db")
	_ = os.MkdirAll(dbDir, 0o755)
	os.Setenv("AUDIO_TAGGER_DB", filepath.Join(dbDir, "app.db"))
}
