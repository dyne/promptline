package main

import (
	"os"

	"github.com/rs/zerolog"
	"promptline/internal/config"
	"promptline/internal/sandbox"
	"promptline/internal/tools"
)

// initSandbox starts the sandbox manager at startup to surface errors early.
// Returns the manager so callers can close it on shutdown.
func initSandbox(cfg *config.Config, logger zerolog.Logger) *sandbox.Manager {
	if !cfg.Sandbox.Enabled {
		logger.Info().Msg("Sandbox disabled via config")
		return nil
	}

	workdir := cfg.Sandbox.Workdir
	if workdir == "" {
		cwd, err := os.Getwd()
		if err == nil {
			workdir = cwd
		} else {
			logger.Warn().Err(err).Msg("Falling back to current directory for sandbox workdir")
			workdir = "."
		}
	}

	mgr := sandbox.NewManager(cfg.Sandbox)
	if err := mgr.Start(); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize sandbox; falling back to host execution")
		return nil
	}

	tools.SetSandboxRunner(mgr, workdir)
	logger.Info().Msg("Sandbox initialized")
	return mgr
}
