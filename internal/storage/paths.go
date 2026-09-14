package storage

import (
	"os"
	"path/filepath"
)

type Paths struct {
	ConfigDir           string
	ConfigFile          string
	StateDir            string
	StateFile           string
	LogFile             string
	CacheDir            string
	LastRenderedImage   string
	DownloadedImage     string
	MaintenanceSentinel string
}

func DefaultPaths() (Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(homeDir, ".config")
	}

	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(homeDir, ".local", "state")
	}

	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		cacheHome = filepath.Join(homeDir, ".cache")
	}

	configDir := filepath.Join(configHome, "trmnl-rm1")
	stateDir := filepath.Join(stateHome, "trmnl-rm1")
	cacheDir := filepath.Join(cacheHome, "trmnl-rm1")

	return Paths{
		ConfigDir:           configDir,
		ConfigFile:          filepath.Join(configDir, "config.json"),
		StateDir:            stateDir,
		StateFile:           filepath.Join(stateDir, "state.json"),
		LogFile:             filepath.Join(stateDir, "cycles.log"),
		CacheDir:            cacheDir,
		LastRenderedImage:   filepath.Join(stateDir, "current.png"),
		DownloadedImage:     filepath.Join(cacheDir, "downloaded.png"),
		MaintenanceSentinel: filepath.Join(configDir, "maintenance"),
	}, nil
}

func EnsureRuntimeDirs(paths Paths) error {
	for _, dir := range []string{paths.ConfigDir, paths.StateDir, paths.CacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
