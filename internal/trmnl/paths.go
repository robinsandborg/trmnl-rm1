package trmnl

import "github.com/robinsandborg/rm1-trmnl/internal/storage"

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

func defaultPaths() (Paths, error)    { p, err := storage.DefaultPaths(); return Paths(p), err }
func ensureRuntimeDirs(p Paths) error { return storage.EnsureRuntimeDirs(storage.Paths(p)) }
