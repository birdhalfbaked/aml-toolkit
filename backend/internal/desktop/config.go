package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ConfigFile is persisted as config.json under [StateDir].
type ConfigFile struct {
	LibraryRoot        string `json:"libraryRoot"`
	OnboardingComplete bool   `json:"onboardingComplete"`
}

// StateDir returns the per-user application directory (e.g. %AppData%\audio-tagger on Windows).
func StateDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, appDirName), nil
}

// RecommendedLibraryDir suggests a first-run folder for project audio (Documents).
func RecommendedLibraryDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// English-friendly default; user can change in the welcome screen.
	return filepath.Join(home, "Documents", "Audio Tagger Library"), nil
}

// ConfigPath returns the path to config.json.
func ConfigPath() (string, error) {
	st, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(st, "config.json"), nil
}

// ReadConfig loads config.json; missing file is a zero config.
func ReadConfig() (ConfigFile, error) {
	var zero ConfigFile
	p, err := ConfigPath()
	if err != nil {
		return zero, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return zero, nil
		}
		return zero, err
	}
	var c ConfigFile
	if err := json.Unmarshal(b, &c); err != nil {
		return zero, err
	}
	return c, nil
}

// WriteConfig replaces config.json.
func WriteConfig(c ConfigFile) error {
	p, err := ConfigPath()
	if err != nil {
		return err
	}
	st, err := StateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(st, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// ResolveDBPath picks the SQLite file: prefers db/app.db, falls back to legacy data/app.db.
func ResolveDBPath(st string) string {
	newP := filepath.Join(st, "db", "app.db")
	if _, err := os.Stat(newP); err == nil {
		return newP
	}
	legacy := filepath.Join(st, "data", "app.db")
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return newP
}

// PrepareDesktopPaths returns DB path, initial library root, and whether the welcome flow must run.
func PrepareDesktopPaths() (dbPath string, libraryDir string, needsOnboarding bool, err error) {
	st, err := StateDir()
	if err != nil {
		return "", "", false, err
	}
	if err = os.MkdirAll(filepath.Join(st, "db"), 0o755); err != nil {
		return "", "", false, err
	}
	dbPath = ResolveDBPath(st)

	cfg, err := ReadConfig()
	if err != nil {
		return "", "", false, err
	}
	if cfg.OnboardingComplete && filepath.Clean(cfg.LibraryRoot) != "" {
		abs, e := filepath.Abs(cfg.LibraryRoot)
		if e != nil {
			return "", "", false, e
		}
		return dbPath, abs, false, nil
	}

	// Legacy layout: single "data" folder held DB + projects (pre split-library).
	legacyData := filepath.Join(st, "data")
	if hasDir(filepath.Join(legacyData, "projects")) {
		abs, e := filepath.Abs(legacyData)
		if e != nil {
			return "", "", false, e
		}
		_ = WriteConfig(ConfigFile{LibraryRoot: abs, OnboardingComplete: true})
		return dbPath, abs, false, nil
	}

	rec, err := RecommendedLibraryDir()
	if err != nil {
		return "", "", false, err
	}
	if err = os.MkdirAll(rec, 0o755); err != nil {
		return "", "", false, err
	}
	abs, err := filepath.Abs(rec)
	if err != nil {
		return "", "", false, err
	}
	return dbPath, abs, true, nil
}

func hasDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// BootstrapStatusPayload is returned by GET /api/bootstrap/status (JSON for SPA).
type BootstrapStatusPayload struct {
	NeedsOnboarding         bool   `json:"needsOnboarding"`
	LibraryRoot             string `json:"libraryRoot"`
	RecommendedLibraryRoot  string `json:"recommendedLibraryRoot"`
	StateDir                string `json:"stateDir"`
	DatabasePath            string `json:"databasePath"`
	DesktopBootstrapEnabled bool   `json:"desktopBootstrapEnabled"`
}

// BootstrapStatusJSON builds GET /api/bootstrap/status body. apiLocked means desktop onboarding is not finished.
func BootstrapStatusJSON(dbPath, libraryDir string, apiLocked bool) (BootstrapStatusPayload, error) {
	st, err := StateDir()
	if err != nil {
		return BootstrapStatusPayload{}, err
	}
	rec, err := RecommendedLibraryDir()
	if err != nil {
		return BootstrapStatusPayload{}, err
	}
	d := IsDesktopBuild()
	return BootstrapStatusPayload{
		NeedsOnboarding:         d && apiLocked,
		LibraryRoot:             libraryDir,
		RecommendedLibraryRoot:  rec,
		StateDir:                st,
		DatabasePath:            dbPath,
		DesktopBootstrapEnabled: d,
	}, nil
}

// IsDesktopBuild reports whether the app runs with desktop bootstrap wiring.
func IsDesktopBuild() bool {
	return os.Getenv("AUDIO_TAGGER_DESKTOP") == "1"
}
