package trmnl

import (
	"context"
	"github.com/robinsandborg/rm1-trmnl/internal/network"
	"github.com/robinsandborg/rm1-trmnl/internal/storage"
	"net/http"
	"os"
	"time"
)

// cycleDeps keeps device effects and writes replaceable for cycle contract
// tests. Configuration, HTTP decoding, hashing, and state transitions still
// run through the production path. Each App owns its dependencies.
type cycleDeps struct {
	ensureInterface      func(Config)
	readBatterySample    func(Config) (*BatterySample, error)
	determineRuntimeMode func(Paths, Config, State, time.Time) (RuntimeMode, error)
	prepareNetwork       func(Config) (*http.Client, func(), error)
	renderImage          func(Config, []byte, string, RefreshMode) error
	planNextCycle        func(Config, time.Duration, RuntimeMode) (RuntimeMode, error)
	suspendDevice        func(Config) error
	writeFile            func(string, []byte, os.FileMode) error
	appendCycleLog       func(Paths, CycleLog) error
	saveState            func(Paths, State) error
}

func defaultCycleDeps(prepare func(Config) (*http.Client, func(), error)) cycleDeps {
	return cycleDeps{
		ensureInterface:      func(cfg Config) { network.EnsureInterface(networkOptions(cfg)) },
		readBatterySample:    readBatterySample,
		determineRuntimeMode: determineRuntimeMode,
		prepareNetwork:       prepare,
		renderImage:          renderImage,
		planNextCycle:        planNextCycle,
		suspendDevice:        suspendDevice,
		writeFile:            storage.WriteFile,
		appendCycleLog:       appendCycleLog,
		saveState:            saveState,
	}
}

// networkDeps exercises the real acquisition/cleanup lifecycle without
// controlling the host's network or waiting for an actual connection.
type networkDeps struct {
	bringUp   func(Config) error
	bringDown func(Config) error
	wait      func(context.Context, Config) error
}
