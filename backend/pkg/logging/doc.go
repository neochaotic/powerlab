// Package logging is PowerLab's structured logger, built on log/slog.
//
// Every backend service constructs a Logger from a Config populated by
// environment variables, then logs through the Logger interface.
// Correlation IDs from pkg/tracing are auto-injected when a request
// context is passed in.
//
// Example:
//
//	cfg := logging.Config{Level: "info", Format: "json"}
//	logger, err := logging.New(cfg)
//	if err != nil {
//	    return err
//	}
//	logger.Info(ctx, "service started", slog.String("addr", addr))
//
// See ADR-0012 for the rationale behind slog over zap.
package logging
